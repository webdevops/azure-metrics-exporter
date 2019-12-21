package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

func probeMetricsResourceHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64

	startTime := time.Now()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, PROBE_METRICS_RESOURCE_TIMEOUT_DEFAULT)
	if err != nil {
		Logger.Error(err)
		http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	var settings RequestMetricSettings
	if val, err := NewRequestMetricSettings(r); err == nil {
		settings = val
	} else {
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(settings.Subscriptions) != 1 {
		err := errors.New("Invalid subscription, one subscription needs to be specified")
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	subscription := settings.Subscriptions[0]

	if settings.Target != "" {
		err := errors.New("Invalid target or target empty")
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	registry, metricGauge := azureInsightMetrics.CreatePrometheusRegistryAndMetricsGauge(settings.Name)

	result, err := azureInsightMetrics.FetchMetrics(ctx, subscription, settings.Target, settings)

	if err != nil {
		Logger.Warningln(err)
		prometheusMetricRequests.With(prometheus.Labels{
			"subscriptionID": subscription,
			"handler":        PROBE_METRICS_RESOURCE_URL,
			"filter":         "",
			"result":         "error",
		}).Inc()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	Logger.Verbosef("subscription[%v] fetched metrics for %v", subscription, settings.Target)
	prometheusMetricRequests.With(prometheus.Labels{
		"subscriptionID": subscription,
		"handler":        PROBE_METRICS_RESOURCE_URL,
		"filter":         "",
		"result":         "success",
	}).Inc()
	result.SetGauge(metricGauge, settings)

	// global stats counter
	prometheusCollectTime.With(prometheus.Labels{
		"subscriptionID": subscription,
		"handler":        PROBE_METRICS_RESOURCE_URL,
		"filter":         "",
	}).Observe(time.Now().Sub(startTime).Seconds())

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
