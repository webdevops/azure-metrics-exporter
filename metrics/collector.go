package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type metricListCollectorDetails struct {
	gauge prometheus.Gauge
	desc  *prometheus.Desc
	ts    time.Time
}

type metricListCollector struct {
	details []*metricListCollectorDetails
}

func NewMetricListCollector(list *MetricList) *metricListCollector {
	collector := &metricListCollector{
		details: []*metricListCollectorDetails{},
	}

	if list == nil {
		return collector
	}

	// create prometheus metrics and set rows
	for _, metricName := range list.GetMetricNames() {
		gaugeVec := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: metricName,
				Help: list.GetMetricHelp(metricName),
			},
			list.GetMetricLabelNames(metricName),
		)

		for _, metric := range list.GetMetricList(metricName) {
			gauge := gaugeVec.With(metric.Labels)
			gauge.Set(metric.Value)

			desc := prometheus.NewDesc(metricName, list.GetMetricHelp(metricName), []string{}, metric.Labels)

			details := &metricListCollectorDetails{
				gauge,
				desc,
				metric.Timestamp,
			}

			collector.details = append(collector.details, details)
		}
	}

	return collector
}

func (c *metricListCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, detail := range c.details {
		ch <- detail.desc
	}
}

func (c *metricListCollector) Collect(ch chan<- prometheus.Metric) {
	for _, detail := range c.details {
		s := prometheus.NewMetricWithTimestamp(detail.ts, detail.gauge)
		ch <- s
	}
}
