package metrics

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	stringsCommon "github.com/webdevops/go-common/strings"
	"github.com/webdevops/go-common/utils/to"
)

type (
	AzureInsightSubscriptionMetricsResult struct {
		AzureInsightBaseMetricsResult

		subscription *armsubscriptions.Subscription
		Result       *armmonitor.MetricsClientListAtSubscriptionScopeResponse
	}
)

func (r *AzureInsightSubscriptionMetricsResult) SendMetricToChannel(channel chan<- PrometheusMetricResult) {
	if r.Result.Value != nil {
		// DEBUGGING
		// data, _ := json.Marshal(r.Result)
		// fmt.Println(string(data))

		for _, metric := range r.Result.Value {
			if metric.Timeseries != nil {
				for _, timeseries := range metric.Timeseries {
					if timeseries.Data != nil {
						// get dimension name (optional)
						dimensions := map[string]string{}
						resourceId := ""
						if timeseries.Metadatavalues != nil {
							for _, dimensionRow := range timeseries.Metadatavalues {
								dimensionRowName := to.String(dimensionRow.Name.Value)
								dimensionRowValue := to.String(dimensionRow.Value)

								if r.prober.settings.DimensionLowercase {
									dimensionRowValue = strings.ToLower(dimensionRowValue)
								}

								if strings.EqualFold(dimensionRowName, "microsoft.resourceid") {
									resourceId = dimensionRowValue
								} else {
									dimensions[dimensionRowName] = dimensionRowValue
								}
							}
						}

						azureResource, _ := armclient.ParseResourceId(resourceId)

						metricUnit := ""
						if metric.Unit != nil {
							metricUnit = string(*metric.Unit)
						}

						metricLabels := prometheus.Labels{
							"resourceID":       strings.ToLower(resourceId),
							"subscriptionID":   azureResource.Subscription,
							"subscriptionName": to.String(r.subscription.DisplayName),
							"resourceGroup":    azureResource.ResourceGroup,
							"resourceName":     azureResource.ResourceName,
							"metric":           to.String(metric.Name.Value),
							"unit":             metricUnit,
							"interval":         to.String(r.prober.settings.Interval),
							"timespan":         r.prober.settings.Timespan,
							"aggregation":      "",
						}

						// add resource tags as labels
						metricLabels = r.prober.AzureResourceTagManager.AddResourceTagsToPrometheusLabels(r.prober.ctx, metricLabels, resourceId)

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
								labelName := "dimension" + stringsCommon.UppercaseFirst(dimensionName)
								labelName = metricLabelNotAllowedChars.ReplaceAllString(labelName, "")
								metricLabels[labelName] = dimensionValue
							}
						}

						for _, timeseriesData := range timeseries.Data {
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
