package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

type AzureInsightMetrics struct {
	metricsClientCache  map[string]*insights.MetricsClient
	resourceClientCache map[string]*resources.Client

	clientMutex sync.Mutex
}

type AzureInsightMetricsResult struct {
	Result     *insights.Response
	ResourceID *string
}

func NewAzureInsightMetrics() *AzureInsightMetrics {
	ret := AzureInsightMetrics{}
	ret.metricsClientCache = map[string]*insights.MetricsClient{}
	ret.resourceClientCache = map[string]*resources.Client{}

	return &ret
}

func (m *AzureInsightMetrics) MetricsClient(subscriptionId string) *insights.MetricsClient {
	m.clientMutex.Lock()

	if _, ok := m.metricsClientCache[subscriptionId]; !ok {
		client := insights.NewMetricsClient(subscriptionId)
		client.Authorizer = AzureAuthorizer
		m.metricsClientCache[subscriptionId] = &client
	}

	client := m.metricsClientCache[subscriptionId]
	m.clientMutex.Unlock()

	return client
}

func (m *AzureInsightMetrics) ResourcesClient(subscriptionId string) *resources.Client {
	m.clientMutex.Lock()

	if _, ok := m.resourceClientCache[subscriptionId]; !ok {
		client := resources.NewClient(subscriptionId)
		client.Authorizer = AzureAuthorizer
		m.resourceClientCache[subscriptionId] = &client
	}

	client := m.resourceClientCache[subscriptionId]
	m.clientMutex.Unlock()

	return client
}

func (m *AzureInsightMetrics) ListResources(subscriptionId, filter string) (resources.ListResultIterator, error) {
	return m.ResourcesClient(subscriptionId).ListComplete(context.Background(), filter, "", nil)
}

func (m *AzureInsightMetrics) CreatePrometheusMetricsGauge(metricName string) (gauge *prometheus.GaugeVec) {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricName,
		Help: "Azure monitor insight metics",
	}, []string{
		"resourceID",
		"metric",
		"unit",
		"aggregation",
		// deprecated
		"type",
		"data",
	})
}

func (m *AzureInsightMetrics) CreatePrometheusRegistryAndMetricsGauge(metricName string) (*prometheus.Registry, *prometheus.GaugeVec) {
	registry := prometheus.NewRegistry()
	gauge := azureInsightMetrics.CreatePrometheusMetricsGauge(metricName)
	registry.MustRegister(gauge)

	return registry, gauge
}

func (m *AzureInsightMetrics) FetchMetrics(ctx context.Context, subscriptionId, resourceID string, settings RequestMetricSettings) (AzureInsightMetricsResult, error) {
	ret := AzureInsightMetricsResult{}

	result, err := m.MetricsClient(subscriptionId).List(ctx, resourceID, settings.Timespan, settings.Interval, settings.Metric, settings.Aggregation, nil, "", "", insights.Data, "")

	if err == nil {
		ret.Result = &result
		ret.ResourceID = &resourceID
	}

	return ret, err
}

func (r *AzureInsightMetricsResult) SetGauge(gauge *prometheus.GaugeVec, settings RequestMetricSettings) {
	if r.Result.Value != nil {
		for _, metric := range *r.Result.Value {
			if metric.Timeseries != nil {
				for _, timeseries := range *metric.Timeseries {
					if timeseries.Data != nil {
						for _, timeseriesData := range *timeseries.Data {
							if timeseriesData.Total != nil {
								gauge.With(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      *metric.Name.Value,
									"unit":        string(metric.Unit),
									"aggregation": "total",
									// deprecated
									"type":*metric.Name.Value,
									"data": "total",
								}).Set(*timeseriesData.Total)
							}

							if timeseriesData.Minimum != nil {
								gauge.With(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      *metric.Name.Value,
									"unit":        string(metric.Unit),
									"aggregation": "minimum",
									// deprecated
									"type":*metric.Name.Value,
									"data": "minimum",
								}).Set(*timeseriesData.Minimum)
							}

							if timeseriesData.Maximum != nil {
								gauge.With(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      *metric.Name.Value,
									"unit":        string(metric.Unit),
									"aggregation": "maximum",
									// deprecated
									"type":*metric.Name.Value,
									"data": "maximum",
								}).Set(*timeseriesData.Maximum)
							}

							if timeseriesData.Average != nil {
								gauge.With(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      *metric.Name.Value,
									"unit":        string(metric.Unit),
									"aggregation": "average",
									// deprecated
									"type":*metric.Name.Value,
									"data": "average",
								}).Set(*timeseriesData.Average)
							}

							if timeseriesData.Count != nil {
								gauge.With(prometheus.Labels{
									"resourceID":  *r.ResourceID,
									"metric":      *metric.Name.Value,
									"unit":        string(metric.Unit),
									"aggregation": "count",
									// deprecated
									"type":*metric.Name.Value,
									"data": "count",
								}).Set(*timeseriesData.Count)
							}
						}
					}
				}
			}
		}
	}
}
