package main

import (
	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
)

const (
	Author  = "webdevops.io"
	Version = "0.5.1"

	METRICS_URL = "/metrics"

	PROBE_METRICS_RESOURCE_URL             = "/probe/metrics/resource"
	PROBE_METRICS_RESOURCE_TIMEOUT_DEFAULT = 10

	PROBE_METRICS_LIST_URL             = "/probe/metrics/list"
	PROBE_METRICS_LIST_TIMEOUT_DEFAULT = 120

	PROBE_METRICS_SCRAPE_URL             = "/probe/metrics/scrape"
	PROBE_METRICS_SCRAPE_TIMEOUT_DEFAULT = 120

	PROBE_LOGANALYTICS_SCRAPE_URL             = "/probe/loganalytics/query"
	PROBE_LOGANALYTICS_SCRAPE_TIMEOUT_DEFAULT = 120
)

var (
	argparser       *flags.Parser
	args            []string
	Verbose         bool
	Logger          *DaemonLogger
	AzureAuthorizer autorest.Authorizer

	prometheusCollectTime *prometheus.SummaryVec

	azureInsightMetrics      *AzureInsightMetrics
	azureLogAnalyticsMetrics *AzureLogAnalysticsMetrics
)

var opts struct {
	// general settings
	Verbose []bool `long:"verbose" short:"v" env:"VERBOSE"      description:"Verbose mode"`

	// server settings
	ServerBind string `long:"bind" env:"SERVER_BIND"  description:"Server address"  default:":8080"`
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

	azureInsightMetrics = NewAzureInsightMetrics()
	azureLogAnalyticsMetrics = NewAzureLogAnalysticsMetrics()
}

// start and handle prometheus handler
func startHttpServer() {
	http.Handle(METRICS_URL, promhttp.Handler())

	http.HandleFunc(PROBE_METRICS_RESOURCE_URL, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsResourceHandler(w, r)
	})

	http.HandleFunc(PROBE_METRICS_LIST_URL, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsListHandler(w, r)
	})

	http.HandleFunc(PROBE_METRICS_SCRAPE_URL, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsScrapeHandler(w, r)
	})

	http.HandleFunc(PROBE_LOGANALYTICS_SCRAPE_URL, func(w http.ResponseWriter, r *http.Request) {
		probeLogAnalyticsQueryHandler(w, r)
	})

	Logger.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

func initMetricCollector() {
	prometheusCollectTime = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "azurerm_stats_metric_collecttime",
			Help: "Azure Insights stats collecttime",
		},
		[]string{
			"subscriptionID",
			"handler",
			"filter",
		},
	)
	prometheus.MustRegister(prometheusCollectTime)
}
