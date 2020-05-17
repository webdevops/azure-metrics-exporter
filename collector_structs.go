package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

type MetricCollectorRow struct {
	labels prometheus.Labels
	value  float64
}

type MetricCollectorList struct {
	list []MetricCollectorRow
	mux sync.Mutex
}

func (m *MetricCollectorList) Add(labels prometheus.Labels, value float64) {
	m.mux.Lock()
	defer m.mux.Unlock()

	m.list = append(m.list, MetricCollectorRow{labels: labels, value: value})
}

func (m *MetricCollectorList) AddIfNotZero(labels prometheus.Labels, value float64) {
	m.mux.Lock()
	defer m.mux.Unlock()

	if value != 0 {
		m.list = append(m.list, MetricCollectorRow{labels: labels, value: value})
	}
}

func (m *MetricCollectorList) AddIfGreaterZero(labels prometheus.Labels, value float64) {
	m.mux.Lock()
	defer m.mux.Unlock()

	if value > 0 {
		m.list = append(m.list, MetricCollectorRow{labels: labels, value: value})
	}
}

func (m *MetricCollectorList) GaugeSet(gauge *prometheus.GaugeVec) {
	m.mux.Lock()
	defer m.mux.Unlock()

	for _, metric := range m.list {
		gauge.With(metric.labels).Set(metric.value)
	}
}
