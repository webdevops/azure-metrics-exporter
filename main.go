package main

import (
	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/jessevdk/go-flags"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-metrics-exporter/config"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

const (
	Author = "webdevops.io"

	MetricsUrl = "/metrics"

	PROMETHEUS_METRIC_NAME = "azurerm_resource_metric"

	ProbeMetricsResourceUrl            = "/probe/metrics/resource"
	ProbeMetricsResourceTimeoutDefault = 10

	ProbeMetricsListUrl            = "/probe/metrics/list"
	ProbeMetricsListTimeoutDefault = 120

	ProbeMetricsScrapeUrl            = "/probe/metrics/scrape"
	ProbeMetricsScrapeTimeoutDefault = 120

	ProbeLoganalyticsScrapeUrl            = "/probe/loganalytics/query"
	ProbeLoganalyticsScrapeTimeoutDefault = 120
)

var (
	argparser *flags.Parser
	opts      config.Opts

	AzureEnvironment   azure.Environment
	AzureAuthorizer    autorest.Authorizer
	AzureAdResourceUrl string

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

	// verbose level
	if opts.Logger.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	// debug level
	if opts.Logger.Debug {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
		log.SetFormatter(&log.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}

	// json log format
	if opts.Logger.LogJson {
		log.SetReportCaller(true)
		log.SetFormatter(&log.JSONFormatter{
			DisableTimestamp: true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}
}

// Init and build Azure authorzier
func initAzureConnection() {
	var err error

	AzureEnvironment, err = azure.EnvironmentFromName(*opts.Azure.Environment)
	if err != nil {
		log.Panic(err)
	}

	if opts.Azure.AdResourceUrl == nil {
		AzureAdResourceUrl = AzureEnvironment.ResourceManagerEndpoint
	} else {
		AzureAdResourceUrl = *opts.Azure.AdResourceUrl
	}

	// setup azure authorizer
	AzureAuthorizer, err = auth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Panic(err)
	}

}

// start and handle prometheus handler
func startHttpServer() {
	http.Handle(MetricsUrl, promhttp.Handler())

	http.HandleFunc(ProbeMetricsResourceUrl, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsResourceHandler(w, r)
	})

	http.HandleFunc(ProbeMetricsListUrl, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsListHandler(w, r)
	})

	http.HandleFunc(ProbeMetricsScrapeUrl, func(w http.ResponseWriter, r *http.Request) {
		probeMetricsScrapeHandler(w, r)
	})

	http.HandleFunc(ProbeLoganalyticsScrapeUrl, func(w http.ResponseWriter, r *http.Request) {
		probeLogAnalyticsQueryHandler(w, r)
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
