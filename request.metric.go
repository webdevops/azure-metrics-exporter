package main

import (
	"net/http"
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

	return ret, nil
}
