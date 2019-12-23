package main

import (
	"net/http"
	"strconv"
	"strings"
)

type (
	RequestMetricSettings struct {
		Name          string
		Subscriptions []string
		Filter        string
		Timespan      string
		Interval      *string
		Metric        string
		Aggregation   string
		Target        string

		// needed for dimension support
		MetricTop     *int32
		MetricFilter  string
		MetricOrderBy string
	}
)

func NewRequestMetricSettings(r *http.Request) (RequestMetricSettings, error) {
	ret := RequestMetricSettings{}
	params := r.URL.Query()

	// param name
	ret.Name = paramsGetWithDefault(params, "name", PROMETHEUS_METRIC_NAME)

	// param subscription
	if subscriptionList, err := paramsGetRequired(params, "subscription"); err == nil {
		subscriptionList := strings.Split(subscriptionList, ",")
		for _, subscription := range subscriptionList {
			subscription = strings.TrimSpace(subscription)
			ret.Subscriptions = append(ret.Subscriptions, subscription)
		}
	} else {
		return ret, err
	}

	// param filter
	if filter, err := paramsGetRequired(params, "filter"); err == nil {
		ret.Filter = filter
	} else {
		return ret, err
	}

	// param timespan
	ret.Timespan = paramsGetWithDefault(params, "timespan", "PT1M")

	// param interval
	if val := params.Get("interval"); val != "" {
		ret.Interval = &val
	}

	// param metric
	ret.Metric = paramsGetWithDefault(params, "metric", "")

	// param aggregation
	ret.Aggregation = paramsGetWithDefault(params, "aggregation", "")

	// param target
	ret.Target = paramsGetWithDefault(params, "target", "")

	// param metricTop
	if val := params.Get("metricTop"); val != "" {
		valInt64, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return ret, err
		}
		valInt32 := int32(valInt64)
		ret.MetricTop = &valInt32
	}

	// param metricFilter
	ret.MetricFilter = paramsGetWithDefault(params, "metricFilter", "")

	// param metricOrderBy
	ret.MetricOrderBy = paramsGetWithDefault(params, "metricOrderBy", "")

	return ret, nil
}
