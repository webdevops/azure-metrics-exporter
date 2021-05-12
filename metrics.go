package main

import "github.com/prometheus/client_golang/prometheus"

type (
	MetricList struct {
		List map[string][]MetricRow
	}

	MetricRow struct {
		Labels prometheus.Labels
		Value  float64
	}
)

func NewMetricList() *MetricList {
	list := MetricList{}
	list.Init()
	return &list
}

func (l *MetricList) Init() {
	l.List = map[string][]MetricRow{}
}

func (l *MetricList) Add(name string, metric ...MetricRow) {
	if _, ok := l.List[name]; !ok {
		l.List[name] = []MetricRow{}
	}

	l.List[name] = append(l.List[name], metric...)
}

func (l *MetricList) GetMetricNames() []string {
	list := []string{}

	for name := range l.List {
		list = append(list, name)
	}

	return list
}

func (l *MetricList) GetMetricList(name string) []MetricRow {
	return l.List[name]
}

func (l *MetricList) GetMetricLabelNames(name string) []string {
	uniqueLabelMap := map[string]string{}

	for _, row := range l.List[name] {
		for labelName := range row.Labels {
			uniqueLabelMap[labelName] = labelName
		}
	}

	list := []string{}
	for labelName := range uniqueLabelMap {
		list = append(list, labelName)
	}

	return list
}
