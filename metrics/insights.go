package metrics

import (
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"strings"
)

var (
	metricNamePlaceholders    = regexp.MustCompile(`{([^}]+)}`)
	metricNameNotAllowedChars = regexp.MustCompile(`[^a-zA-Z0-9_:]`)
)

type (
	AzureInsightMetrics struct {
	}

	AzureInsightMetricsResult struct {
		settings   *RequestMetricSettings
		Result     *insights.Response
		ResourceID *string
	}

	PrometheusMetricResult struct {
		Name   string
		Labels prometheus.Labels
		Value  float64
	}
)

func (p *MetricProber) MetricsClient(subscriptionId string) *insights.MetricsClient {
	client := insights.NewMetricsClientWithBaseURI(p.Azure.Environment.ResourceManagerEndpoint, subscriptionId)
	client.Authorizer = p.Azure.AzureAuthorizer
	return &client
}

func (p *MetricProber) FetchMetricsFromTarget(client *insights.MetricsClient, target MetricProbeTarget) (AzureInsightMetricsResult, error) {
	ret := AzureInsightMetricsResult{
		settings: p.settings,
	}

	result, err := client.List(
		p.ctx,
		target.ResourceId+p.settings.ResourceSubPath,
		p.settings.Timespan,
		p.settings.Interval,
		strings.Join(target.Metrics, ","),
		strings.Join(target.Aggregations, ","),
		p.settings.MetricTop,
		p.settings.MetricOrderBy,
		p.settings.MetricFilter,
		insights.Data,
		p.settings.MetricNamespace,
	)

	if err == nil {
		if result.Request.URL != nil {
			p.logger.Debugf("sent request to %s", result.Request.URL.String())
		}
		ret.Result = &result
		ret.ResourceID = &target.ResourceId
	}

	return ret, err
}

func (r *AzureInsightMetricsResult) buildMetric(labels prometheus.Labels, value float64) (metric PrometheusMetricResult) {
	metric = PrometheusMetricResult{
		Name:   r.settings.MetricTemplate,
		Labels: labels,
		Value:  value,
	}

	// fallback if template is empty (should not be)
	if r.settings.Name == "" {
		metric.Name = r.settings.Name
	}

	if metricNamePlaceholders.MatchString(metric.Name) {
		metric.Name = metricNamePlaceholders.ReplaceAllStringFunc(
			metric.Name,
			func(fieldName string) string {
				fieldName = strings.Trim(fieldName, "{}")
				switch fieldName {
				case "name":
					return r.settings.Name
				default:
					if fieldValue, exists := metric.Labels[fieldName]; exists {
						delete(metric.Labels, fieldName)
						return fieldValue
					}
				}
				return ""
			},
		)
	}

	metric.Name = strings.ReplaceAll(metric.Name, "-", "_")
	metric.Name = strings.ReplaceAll(metric.Name, " ", "_")
	metric.Name = strings.ToLower(metric.Name)
	metric.Name = metricNameNotAllowedChars.ReplaceAllString(metric.Name, "")

	return
}

func (r *AzureInsightMetricsResult) SendMetricToChannel(channel chan<- PrometheusMetricResult) {
	if r.Result.Value != nil {
		// DEBUGGING
		//data, _ := json.Marshal(r.Result)
		//fmt.Println(string(data))

		for _, metric := range *r.Result.Value {
			if metric.Timeseries != nil {
				for _, timeseries := range *metric.Timeseries {
					if timeseries.Data != nil {
						for dataIndex, timeseriesData := range *timeseries.Data {
							// get dimension name (optional)
							dimensionName := ""
							if timeseries.Metadatavalues != nil {
								if len(*timeseries.Metadatavalues)-1 >= dataIndex {
									dimensionName = *(*timeseries.Metadatavalues)[dataIndex].Value
								}
							}

							resourceId := to.String(r.ResourceID)
							if r.settings.LowercaseResourceId {
								resourceId = strings.ToLower(resourceId)
							}

							if timeseriesData.Total != nil {
								channel <- r.buildMetric(
									prometheus.Labels{
										"resourceID":  resourceId,
										"metric":      to.String(metric.Name.Value),
										"dimension":   dimensionName,
										"unit":        string(metric.Unit),
										"aggregation": "total",
										"interval":    to.String(r.settings.Interval),
										"timespan":    r.settings.Timespan,
									},
									*timeseriesData.Total,
								)
							}

							if timeseriesData.Minimum != nil {
								channel <- r.buildMetric(
									prometheus.Labels{
										"resourceID":  resourceId,
										"metric":      to.String(metric.Name.Value),
										"dimension":   dimensionName,
										"unit":        string(metric.Unit),
										"aggregation": "minimum",
										"interval":    to.String(r.settings.Interval),
										"timespan":    r.settings.Timespan,
									},
									*timeseriesData.Minimum,
								)
							}

							if timeseriesData.Maximum != nil {
								channel <- r.buildMetric(
									prometheus.Labels{
										"resourceID":  resourceId,
										"metric":      to.String(metric.Name.Value),
										"dimension":   dimensionName,
										"unit":        string(metric.Unit),
										"aggregation": "maximum",
										"interval":    to.String(r.settings.Interval),
										"timespan":    r.settings.Timespan,
									},
									*timeseriesData.Maximum,
								)
							}

							if timeseriesData.Average != nil {
								channel <- r.buildMetric(
									prometheus.Labels{
										"resourceID":  resourceId,
										"metric":      to.String(metric.Name.Value),
										"dimension":   dimensionName,
										"unit":        string(metric.Unit),
										"aggregation": "average",
										"interval":    to.String(r.settings.Interval),
										"timespan":    r.settings.Timespan,
									},
									*timeseriesData.Average,
								)
							}

							if timeseriesData.Count != nil {
								channel <- r.buildMetric(
									prometheus.Labels{
										"resourceID":  resourceId,
										"metric":      to.String(metric.Name.Value),
										"dimension":   dimensionName,
										"unit":        string(metric.Unit),
										"aggregation": "count",
										"interval":    to.String(r.settings.Interval),
										"timespan":    r.settings.Timespan,
									},
									*timeseriesData.Count,
								)
							}
						}
					}
				}
			}
		}
	}
}
