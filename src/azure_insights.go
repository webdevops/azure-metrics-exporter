package main

import (
	"sync"
	"context"
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/prometheus/client_golang/prometheus"
)

type AzureInsightMetrics struct {
	metricsClientCache map[string]*insights.MetricsClient
	resourceClientCache map[string]*resources.Client

	clientMutex sync.Mutex
}

type AzureInsightMetricsResult struct {
	Result *insights.Response
	ResourceID *string
}

func NewAzureInsightMetrics() *AzureInsightMetrics {
	ret := AzureInsightMetrics{}
	ret.metricsClientCache = map[string]*insights.MetricsClient{}
	ret.resourceClientCache = map[string]*resources.Client{}

	return &ret
}

func (m *AzureInsightMetrics) MetricsClient(subscriptionId string) *insights.MetricsClient {
	if _, ok := m.metricsClientCache[subscriptionId]; !ok {
		m.clientMutex.Lock()
		client := insights.NewMetricsClient(subscriptionId)
		client.Authorizer = AzureAuthorizer
		m.metricsClientCache[subscriptionId] = &client
		m.clientMutex.Unlock()
	}

	return m.metricsClientCache[subscriptionId]
}
func (m *AzureInsightMetrics) ResourcesClient(subscriptionId string) *resources.Client {
	if _, ok := m.resourceClientCache[subscriptionId]; !ok {
		m.clientMutex.Lock()
		client := resources.NewClient(subscriptionId)
		client.Authorizer = AzureAuthorizer
		m.resourceClientCache[subscriptionId] = &client
		m.clientMutex.Unlock()
	}

	return m.resourceClientCache[subscriptionId]
}

func (m *AzureInsightMetrics) ListResources(subscriptionId, filter string) (resources.ListResultIterator, error) {
	return  m.ResourcesClient(subscriptionId).ListComplete(context.Background(), filter, "", nil)
}

func (m *AzureInsightMetrics) CreatePrometheusMetricsGauge() (gauge *prometheus.GaugeVec) {
	gauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "azurerm_resource_metric",
		Help: "Azure monitor insight metics",
	}, []string{
		"resourceID",
		"type",
		"unit",
		"data",
	})

	return
}

func (m *AzureInsightMetrics) CreatePrometheusRegistryAndMetricsGauge() (*prometheus.Registry, *prometheus.GaugeVec) {
	registry := prometheus.NewRegistry()
	gauge := azureInsightMetrics.CreatePrometheusMetricsGauge()
	registry.MustRegister(gauge)

	return registry, gauge
}

func (m *AzureInsightMetrics) FetchMetrics(ctx context.Context, subscriptionId, resourceID, timespan string, interval *string, metric, aggregation string) (AzureInsightMetricsResult, error) {
	ret := AzureInsightMetricsResult{}

	result, err := m.MetricsClient(subscriptionId).List(ctx, resourceID, timespan, interval, metric, aggregation, nil, "", "", insights.Data, "")

	if err == nil {
		ret.Result = &result
		ret.ResourceID = &resourceID
	}

	return ret, err
}

func (r *AzureInsightMetricsResult) SetGauge(gauge *prometheus.GaugeVec) {
	if r.Result.Value != nil {
		for _, metric := range *r.Result.Value {
			if metric.Timeseries != nil {
				for _, timeseries := range *metric.Timeseries {
					if timeseries.Data != nil {
						for _, timeseriesData := range *timeseries.Data {
							if timeseriesData.Total != nil {
								gauge.With(prometheus.Labels{
									"resourceID": *r.ResourceID,
									"type":       *metric.Name.Value,
									"unit":       string(metric.Unit),
									"data":       "total",
								}).Set(*timeseriesData.Total)
							}

							if timeseriesData.Minimum != nil {
								gauge.With(prometheus.Labels{
									"resourceID": *r.ResourceID,
									"type":       *metric.Name.Value,
									"unit":       string(metric.Unit),
									"data":       "minimum",
								}).Set(*timeseriesData.Minimum)
							}

							if timeseriesData.Maximum != nil {
								gauge.With(prometheus.Labels{
									"resourceID": *r.ResourceID,
									"type":       *metric.Name.Value,
									"unit":       string(metric.Unit),
									"data":       "maximum",
								}).Set(*timeseriesData.Maximum)
							}

							if timeseriesData.Average != nil {
								gauge.With(prometheus.Labels{
									"resourceID": *r.ResourceID,
									"type":       *metric.Name.Value,
									"unit":       string(metric.Unit),
									"data":       "average",
								}).Set(*timeseriesData.Average)
							}
						}
					}
				}
			}
		}
	}
}
