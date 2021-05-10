package main

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/Azure/go-autorest/autorest"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type (
	AzureInsightMetrics struct {
		authorizer         *autorest.Authorizer
		prometheusRegistry *prometheus.Registry

		prometheus struct {
			apiQuota *prometheus.GaugeVec
		}
	}

	AzureInsightMetricsResult struct {
		Result     *insights.Response
		ResourceID *string
	}

	AzureResource struct {
		ID   *string
		Tags map[string]*string
	}
)

func NewAzureInsightMetrics(authorizer autorest.Authorizer, registry *prometheus.Registry) *AzureInsightMetrics {
	ret := AzureInsightMetrics{}
	ret.authorizer = &authorizer
	ret.prometheusRegistry = registry

	ret.prometheus.apiQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_ratelimit",
			Help: "Azure ResourceManager ratelimit",
		},
		[]string{
			"subscriptionID",
			"scope",
			"type",
		},
	)
	ret.prometheusRegistry.MustRegister(ret.prometheus.apiQuota)

	return &ret
}

func (m *AzureInsightMetrics) MetricsClient(subscriptionId string) *insights.MetricsClient {
	client := insights.NewMetricsClientWithBaseURI(AzureAdResourceUrl, subscriptionId)
	client.Authorizer = *m.authorizer
	client.ResponseInspector = m.azureResponseInsepector(subscriptionId)

	return &client
}

func (m *AzureInsightMetrics) ResourcesClient(subscriptionId string) *resources.Client {
	client := resources.NewClientWithBaseURI(AzureAdResourceUrl, subscriptionId)
	client.Authorizer = *m.authorizer
	client.ResponseInspector = m.azureResponseInsepector(subscriptionId)

	return &client
}

func (m *AzureInsightMetrics) azureResponseInsepector(subscriptionId string) autorest.RespondDecorator {
	apiQuotaMetric := func(r *http.Response, headerName string, labels prometheus.Labels) {
		ratelimit := r.Header.Get(headerName)
		if v, err := strconv.ParseInt(ratelimit, 10, 64); err == nil {
			m.prometheus.apiQuota.With(labels).Set(float64(v))
		}
	}

	return func(p autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(r *http.Response) error {
			// subscription rate limits
			apiQuotaMetric(r, "x-ms-ratelimit-remaining-subscription-reads", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "read"})
			apiQuotaMetric(r, "x-ms-ratelimit-remaining-subscription-resource-requests", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "resource-requests"})
			apiQuotaMetric(r, "x-ms-ratelimit-remaining-subscription-resource-entities-read", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "resource-entities-read"})

			// tenant rate limits
			apiQuotaMetric(r, "x-ms-ratelimit-remaining-tenant-reads", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "read"})
			apiQuotaMetric(r, "x-ms-ratelimit-remaining-tenant-resource-requests", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "resource-requests"})
			apiQuotaMetric(r, "x-ms-ratelimit-remaining-tenant-resource-entities-read", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "resource-entities-read"})
			return nil
		})
	}
}

func (m *AzureInsightMetrics) ListResources(ctx context.Context, logger *log.Entry, subscriptionId, filter string) ([]AzureResource, error) {
	var cacheDuration *time.Duration
	cacheKey := ""

	resourceList := []AzureResource{}

	if opts.Azure.ServiceDiscovery.CacheDuration != nil && opts.Azure.ServiceDiscovery.CacheDuration.Seconds() > 0 {
		cacheDuration = opts.Azure.ServiceDiscovery.CacheDuration
		cacheKey = fmt.Sprintf(
			"sd:%x",
			string(sha1.New().Sum([]byte(fmt.Sprintf("%v:%v", subscriptionId, filter)))),
		)
	}
	// try cache
	if cacheDuration != nil {
		if v, ok := azureCache.Get(cacheKey); ok {
			if cacheData, ok := v.([]byte); ok {
				if err := json.Unmarshal(cacheData, &resourceList); err == nil {
					logger.Debug("fetched servicediscovery from cache")
					return resourceList, nil
				} else {
					logger.Debug("unable to parse cached servicediscovery")
				}
			}
		}
	}

	list, err := m.ResourcesClient(subscriptionId).ListComplete(context.Background(), filter, "", nil)
	if err != nil {
		return resourceList, err
	}

	for list.NotDone() {
		val := list.Value()

		resourceList = append(
			resourceList,
			AzureResource{
				ID:   val.ID,
				Tags: val.Tags,
			},
		)

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	// store to cache (if enabeld)
	if cacheDuration != nil {
		logger.Debug("saving servicedisccovery to cache")
		if cacheData, err := json.Marshal(resourceList); err == nil {
			azureCache.Set(cacheKey, cacheData, *cacheDuration)
			logger.Debugf("saved servicediscovery to cache for %s", cacheDuration.String())
		}
	}

	return resourceList, nil
}

func (m *AzureInsightMetrics) CreatePrometheusMetricsGauge(metricName string) (gauge *prometheus.GaugeVec) {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricName,
		Help: "Azure monitor insight metics",
	}, []string{
		"resourceID",
		"metric",
		"dimension",
		"unit",
		"aggregation",
		"interval",
		"timespan",
	})
}

func (m *AzureInsightMetrics) CreatePrometheusRegistryAndMetricsGauge(metricName string) *prometheus.GaugeVec {
	gauge := m.CreatePrometheusMetricsGauge(metricName)
	m.prometheusRegistry.MustRegister(gauge)

	return gauge
}

func (m *AzureInsightMetrics) FetchMetrics(ctx context.Context, subscriptionId, resourceID string, settings RequestMetricSettings) (AzureInsightMetricsResult, error) {
	ret := AzureInsightMetricsResult{}

	result, err := m.MetricsClient(subscriptionId).List(
		ctx,
		resourceID,
		settings.Timespan,
		settings.Interval,
		strings.Join(settings.Metric, ","),
		strings.Join(settings.Aggregation, ","),
		settings.MetricTop,
		settings.MetricOrderBy,
		settings.MetricFilter,
		insights.Data,
		"",
	)

	if err == nil {
		ret.Result = &result
		ret.ResourceID = &resourceID
	}

	return ret, err
}

func (r *AzureInsightMetricsResult) SetGauge(gauge *prometheusCommon.MetricList, settings RequestMetricSettings) {
	if r.Result.Value != nil {
		// DEBUGGING
		//data,_ := json.Marshal(r.Result)
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

							if timeseriesData.Total != nil {
								gauge.Add(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      stringPtrToString(metric.Name.Value),
									"dimension":   dimensionName,
									"unit":        string(metric.Unit),
									"aggregation": "total",
									"interval":    stringPtrToString(settings.Interval),
									"timespan":    settings.Timespan,
								}, *timeseriesData.Total)
							}

							if timeseriesData.Minimum != nil {
								gauge.Add(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      stringPtrToString(metric.Name.Value),
									"dimension":   dimensionName,
									"unit":        string(metric.Unit),
									"aggregation": "minimum",
									"interval":    stringPtrToString(settings.Interval),
									"timespan":    settings.Timespan,
								}, *timeseriesData.Minimum)
							}

							if timeseriesData.Maximum != nil {
								gauge.Add(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      stringPtrToString(metric.Name.Value),
									"dimension":   dimensionName,
									"unit":        string(metric.Unit),
									"aggregation": "maximum",
									"interval":    stringPtrToString(settings.Interval),
									"timespan":    settings.Timespan,
								}, *timeseriesData.Maximum)
							}

							if timeseriesData.Average != nil {
								gauge.Add(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      stringPtrToString(metric.Name.Value),
									"dimension":   dimensionName,
									"unit":        string(metric.Unit),
									"aggregation": "average",
									"interval":    stringPtrToString(settings.Interval),
									"timespan":    settings.Timespan,
								}, *timeseriesData.Average)
							}

							if timeseriesData.Count != nil {
								gauge.Add(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      stringPtrToString(metric.Name.Value),
									"dimension":   dimensionName,
									"unit":        string(metric.Unit),
									"aggregation": "count",
									"interval":    stringPtrToString(settings.Interval),
									"timespan":    settings.Timespan,
								}, *timeseriesData.Count)
							}
						}
					}
				}
			}
		}
	}
}
