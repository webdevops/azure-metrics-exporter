package main

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/monitor/mgmt/insights"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	Author  = "webdevops.io"
	Version = "0.2.0"
)

var (
	argparser       *flags.Parser
	args            []string
	Verbose         bool
	Logger          *DaemonLogger
	AzureAuthorizer autorest.Authorizer

	prometheusCollectTime *prometheus.SummaryVec
)

var opts struct {
	// general settings
	Verbose []bool `long:"verbose" short:"v" env:"VERBOSE"      description:"Verbose mode"`

	// server settings
	ServerBind string `long:"bind"              env:"SERVER_BIND"  description:"Server address"  default:":8080"`
}

func main() {
	initArgparser()

	// set verbosity
	Verbose = len(opts.Verbose) >= 1

	Logger = NewLogger(log.Lshortfile, Verbose)
	defer Logger.Close()

	// set verbosity
	Verbose = len(opts.Verbose) >= 1

	Logger.Infof("Init Azure Insights Monitor exporter v%s (written by %v)", Version, Author)

	Logger.Infof("Init Azure connection")
	initAzureConnection()
	initMetricCollector()

	Logger.Infof("Starting http server on %s", opts.ServerBind)
	startHttpServer()
}

// init argparser and parse/validate arguments
func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println(err)
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}
}

// Init and build Azure authorzier
func initAzureConnection() {
	var err error

	// setup azure authorizer
	AzureAuthorizer, err = auth.NewAuthorizerFromEnvironment()
	if err != nil {
		panic(err)
	}
}

func initMetricCollector() {
	prometheusCollectTime = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "azurerm_insights_stats_collecttime",
			Help: "Azure Insights stats collecttime",
		},
		[]string{
			"subscriptionID",
		},
	)

	prometheus.MustRegister(prometheusCollectTime)
}

// start and handle prometheus handler
func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})

	Logger.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

func probeHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	startTime := time.Now()

	// If a timeout is configured via the Prometheus header, add it to the request.
	var timeoutSeconds float64
	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		var err error
		timeoutSeconds, err = strconv.ParseFloat(v, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
			return
		}
	}
	if timeoutSeconds == 0 {
		timeoutSeconds = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	registry := prometheus.NewRegistry()
	metricGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "azurerm_insights_metric",
		Help: "Azure monitor insight metics",
	}, []string{
		"resourceID",
		"type",
		"unit",
		"data",
	})
	registry.MustRegister(metricGauge)

	subscription := params.Get("subscription")
	if subscription == "" {
		http.Error(w, "subscription parameter is missing", http.StatusBadRequest)
		return
	}

	target := params.Get("target")
	if target == "" {
		http.Error(w, "target parameter is missing", http.StatusBadRequest)
		return
	}

	timespan := params.Get("timespan")
	if timespan == "" {
		timespan = "PT1M"
	}

	var interval *string
	if val := params.Get("interval"); val != "" {
		interval = &val
	}

	metric := params.Get("metric")
	aggregation := params.Get("aggregation")

	client := insights.NewMetricsClient(subscription)
	client.Authorizer = AzureAuthorizer

	result, err := client.List(ctx, target, timespan, interval, metric, aggregation, nil, "", "", insights.Data)

	if err != nil {
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	Logger.Verbosef("subscription[%v] fetched metrics for %v", subscription, target)

	if result.Value != nil {
		for _, metric := range *result.Value {
			if metric.Timeseries != nil {
				for _, timeseries := range *metric.Timeseries {
					if timeseries.Data != nil {
						for _, timeseriesData := range *timeseries.Data {
							if timeseriesData.Total != nil {
								metricGauge.With(prometheus.Labels{
									"resourceID": target,
									"type":     *metric.Name.Value,
									"unit":     string(metric.Unit),
									"data":     "total",
								}).Set(*timeseriesData.Total)
							}

							if timeseriesData.Minimum != nil {
								metricGauge.With(prometheus.Labels{
									"resourceID": target,
									"type":     *metric.Name.Value,
									"unit":     string(metric.Unit),
									"data":     "minimum",
								}).Set(*timeseriesData.Minimum)
							}

							if timeseriesData.Maximum != nil {
								metricGauge.With(prometheus.Labels{
									"resourceID": target,
									"type":     *metric.Name.Value,
									"unit":     string(metric.Unit),
									"data":     "maximum",
								}).Set(*timeseriesData.Maximum)
							}

							if timeseriesData.Average != nil {
								metricGauge.With(prometheus.Labels{
									"resourceID": target,
									"type":     *metric.Name.Value,
									"unit":     string(metric.Unit),
									"data":     "average",
								}).Set(*timeseriesData.Average)
							}
						}
					}
				}
			}
		}
	}

	// global stats counter
	prometheusCollectTime.With(prometheus.Labels{
		"subscriptionID": subscription,
	}).Observe( time.Now().Sub(startTime).Seconds() )

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
