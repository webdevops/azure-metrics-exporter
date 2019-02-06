package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

func probeMetricsResourceHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64
	var subscription, target string
	params := r.URL.Query()

	startTime := time.Now()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, PROBE_METRICS_RESOURCE_TIMEOUT_DEFAULT)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	registry, metricGauge := azureInsightMetrics.CreatePrometheusRegistryAndMetricsGauge()

	if subscription, err = paramsGetRequired(params, "subscription"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if target, err = paramsGetRequired(params, "target"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	timespan := paramsGetWithDefault(params, "timespan", "PT1M")

	var interval *string
	if val := params.Get("interval"); val != "" {
		interval = &val
	}

	metric := paramsGetWithDefault(params, "metric", "")
	aggregation := paramsGetWithDefault(params, "aggregation", "")

	result, err := azureInsightMetrics.FetchMetrics(ctx, subscription, target, timespan, interval, metric, aggregation)

	if err != nil {
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	Logger.Verbosef("subscription[%v] fetched metrics for %v", subscription, target)
	result.SetGauge(metricGauge)

	// global stats counter
	prometheusCollectTime.With(prometheus.Labels{
		"subscriptionID": subscription,
		"handler":        PROBE_METRICS_RESOURCE_URL,
		"filter":         "",
	}).Observe(time.Now().Sub(startTime).Seconds())

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
