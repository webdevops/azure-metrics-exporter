package main

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jessevdk/go-flags"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	azureCommon "github.com/webdevops/go-common/azure"
	"github.com/webdevops/go-common/prometheus/azuretracing"

	"github.com/webdevops/azure-metrics-exporter/config"
)

const (
	Author = "webdevops.io"

	UserAgent = "azure-metrics-exporter/"

	MetricsUrl = "/metrics"

	ProbeMetricsResourceUrl            = "/probe/metrics/resource"
	ProbeMetricsResourceTimeoutDefault = 10

	ProbeMetricsListUrl            = "/probe/metrics/list"
	ProbeMetricsListTimeoutDefault = 120

	ProbeMetricsScrapeUrl            = "/probe/metrics/scrape"
	ProbeMetricsScrapeTimeoutDefault = 120

	ProbeMetricsResourceGraphUrl            = "/probe/metrics/resourcegraph"
	ProbeMetricsResourceGraphTimeoutDefault = 120
)

var (
	argparser *flags.Parser
	opts      config.Opts

	AzureClient                *azureCommon.Client
	AzureSubscriptionsIterator *azureCommon.SubscriptionsIterator

	prometheusCollectTime    *prometheus.SummaryVec
	prometheusMetricRequests *prometheus.CounterVec

	metricsCache *cache.Cache
	azureCache   *cache.Cache

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

func main() {
	initArgparser()
	initLogger()

	log.Infof("starting azure-metrics-exporter v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	log.Info(string(opts.GetJson()))
	metricsCache = cache.New(1*time.Minute, 1*time.Minute)
	azureCache = cache.New(1*time.Minute, 1*time.Minute)

	log.Infof("init Azure connection")
	initAzureConnection()
	initMetricCollector()

	log.Infof("starting http server on %s", opts.ServerBind)
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}
}

func initLogger() {
	// verbose level
	if opts.Logger.Debug {
		log.SetLevel(log.DebugLevel)
	}

	// trace level
	if opts.Logger.Trace {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
		log.SetFormatter(&log.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, "/")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", f.File, f.Line)
			},
		})
	}

	// json log format
	if opts.Logger.Json {
		log.SetReportCaller(true)
		log.SetFormatter(&log.JSONFormatter{
			DisableTimestamp: true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, "/")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", f.File, f.Line)
			},
		})
	}
}

func initAzureConnection() {
	var err error
	AzureClient, err = azureCommon.NewClientFromEnvironment(*opts.Azure.Environment, log.StandardLogger())
	if err != nil {
		log.Panic(err.Error())
	}

	AzureClient.SetUserAgent(UserAgent + gitTag)
	AzureSubscriptionsIterator = azureCommon.NewSubscriptionIterator(AzureClient)
}

// start and handle prometheus handler
func startHttpServer() {
	// healthz
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			log.Error(err)
		}
	})

	// readyz
	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			log.Error(err)
		}
	})

	http.Handle(MetricsUrl, azuretracing.RegisterAzureMetricAutoClean(promhttp.Handler()))

	http.HandleFunc(ProbeMetricsResourceUrl, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsResourceHandler(w, r)
	})

	http.HandleFunc(ProbeMetricsListUrl, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsListHandler(w, r)
	})

	http.HandleFunc(ProbeMetricsScrapeUrl, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsScrapeHandler(w, r)
	})

	http.HandleFunc(ProbeMetricsResourceGraphUrl, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsResourceGraphHandler(w, r)
	})

	// report
	reportTmpl := template.Must(template.ParseFiles("./templates/query.html"))
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		cspNonce := base64.StdEncoding.EncodeToString([]byte(uuid.New().String()))

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Referrer-Policy", "same-origin")
		w.Header().Add("X-Frame-Options", "DENY")
		w.Header().Add("X-XSS-Protection", "1; mode=block")
		w.Header().Add("X-Content-Type-Options", "nosniff")
		w.Header().Add("Content-Security-Policy",
			fmt.Sprintf(
				"default-src 'self'; script-src 'nonce-%[1]s'; style-src 'nonce-%[1]s'; img-src 'self' data:",
				cspNonce,
			),
		)

		templatePayload := struct {
			Nonce string
		}{
			Nonce: cspNonce,
		}

		if err := reportTmpl.Execute(w, templatePayload); err != nil {
			log.Error(err)
		}
	})

	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
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

	prometheusMetricRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "azurerm_stats_metric_requests",
			Help: "Azure Insights resource requests",
		},
		[]string{
			"subscriptionID",
			"handler",
			"filter",
			"result",
		},
	)
	prometheus.MustRegister(prometheusMetricRequests)
}
