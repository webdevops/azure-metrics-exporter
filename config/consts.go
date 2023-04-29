package config

const (
	MetricsUrl = "/metrics"

	ProbeMetricsResourceUrl            = "/probe/metrics/resource"
	ProbeMetricsResourceTimeoutDefault = 10

	ProbeMetricsListUrl            = "/probe/metrics/list"
	ProbeMetricsListTimeoutDefault = 120

	ProbeMetricsSubscriptionUrl            = "/probe/metrics"
	ProbeMetricsSubscriptionTimeoutDefault = 120

	ProbeMetricsScrapeUrl            = "/probe/metrics/scrape"
	ProbeMetricsScrapeTimeoutDefault = 120

	ProbeMetricsResourceGraphUrl            = "/probe/metrics/resourcegraph"
	ProbeMetricsResourceGraphTimeoutDefault = 120
)
