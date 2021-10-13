package metrics

import (
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"strings"
)

var (
	metricNamePlaceholders     = regexp.MustCompile(`{([^}]+)}`)
	metricNameNotAllowedChars  = regexp.MustCompile(`[^a-zA-Z0-9_:]`)
	metricLabelNotAllowedChars = regexp.MustCompile(`[^a-zA-Z0-9_]`)
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
		Help   string
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
	// copy map to ensure we don't keep references
	metricLabels := prometheus.Labels{}
	for labelName, labelValue := range labels {
		metricLabels[labelName] = labelValue
	}

	metric = PrometheusMetricResult{
		Name:   r.settings.MetricTemplate,
		Labels: metricLabels,
		Value:  value,
	}

	// fallback if template is empty (should not be)
	if r.settings.Name == "" {
		metric.Name = r.settings.Name
	}

	// set help
	metric.Help = r.settings.HelpTemplate
	if metricNamePlaceholders.MatchString(metric.Help) {
		metric.Help = metricNamePlaceholders.ReplaceAllStringFunc(
			metric.Help,
			func(fieldName string) string {
				fieldName = strings.Trim(fieldName, "{}")
				switch fieldName {
				case "name":
					return r.settings.Name
				default:
					if fieldValue, exists := metric.Labels[fieldName]; exists {
						return fieldValue
					}
				}
				return ""
			},
		)
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
						// remove label, when we add it to metric name
						delete(metric.Labels, fieldName)
						return fieldValue
					}
				}
				return ""
			},
		)
	}

	// sanitize metric name
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
						// get dimension name (optional)
						dimensions := map[string]string{}
						if timeseries.Metadatavalues != nil {
							for _, dimensionRow := range *timeseries.Metadatavalues {
								dimensions[to.String(dimensionRow.Name.Value)] = to.String(dimensionRow.Value)
							}
						}

						resourceId := to.String(r.ResourceID)
						if r.settings.LowercaseResourceId {
							resourceId = strings.ToLower(resourceId)
						}

						subscriptionId := ""
						resourceGroup := ""
						resourceName := ""
						if resourceInfo, err := azure.ParseResourceID(resourceId); err == nil {
							subscriptionId = resourceInfo.SubscriptionID
							resourceGroup = resourceInfo.ResourceGroup
							resourceName = resourceInfo.ResourceName
						}

						metricLabels := prometheus.Labels{
							"resourceID":     resourceId,
							"subscriptionID": subscriptionId,
							"resourceGroup":  resourceGroup,
							"resourceName":   resourceName,
							"metric":         to.String(metric.Name.Value),
							"unit":           string(metric.Unit),
							"interval":       to.String(r.settings.Interval),
							"timespan":       r.settings.Timespan,
							"aggregation":    "",
						}

						if len(dimensions) == 1 {
							// we have only one dimension
							// add one dimension="foobar" label (backward compatibility)
							for _, dimensionValue := range dimensions {
								metricLabels["dimension"] = dimensionValue
							}
						} else if len(dimensions) >= 2 {
							// we have multiple dimensions
							// add each dimension as dimensionXzy="foobar" label
							for dimensionName, dimensionValue := range dimensions {
								labelName := "dimension" + strings.Title(strings.ToLower(dimensionName))
								labelName = metricLabelNotAllowedChars.ReplaceAllString(labelName, "")
								metricLabels[labelName] = dimensionValue
							}
						}

						for _, timeseriesData := range *timeseries.Data {
							if timeseriesData.Total != nil {
								metricLabels["aggregation"] = "total"
								channel <- r.buildMetric(
									metricLabels,
									*timeseriesData.Total,
								)
							}

							if timeseriesData.Minimum != nil {
								metricLabels["aggregation"] = "minimum"
								channel <- r.buildMetric(
									metricLabels,
									*timeseriesData.Minimum,
								)
							}

							if timeseriesData.Maximum != nil {
								metricLabels["aggregation"] = "maximum"
								channel <- r.buildMetric(
									metricLabels,
									*timeseriesData.Maximum,
								)
							}

							if timeseriesData.Average != nil {
								metricLabels["aggregation"] = "average"
								channel <- r.buildMetric(
									metricLabels,
									*timeseriesData.Average,
								)
							}

							if timeseriesData.Count != nil {
								metricLabels["aggregation"] = "count"
								channel <- r.buildMetric(
									metricLabels,
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
