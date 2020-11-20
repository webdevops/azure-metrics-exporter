package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"net/http"
	"time"
)

func probeMetricsResourceHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64

	startTime := time.Now()

	contextLogger := buildContextLoggerFromRequest(r)

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, ProbeMetricsResourceTimeoutDefault)
	if err != nil {
		contextLogger.Error(err)
		http.Error(w, fmt.Sprintf("failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	var settings RequestMetricSettings
	if settings, err = NewRequestMetricSettings(r); err != nil {
		contextLogger.Errorln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(settings.Subscriptions) != 1 {
		contextLogger.Errorln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	subscription := settings.Subscriptions[0]

	if len(settings.Target) == 0 {
		contextLogger.Errorln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	registry, metricGauge := azureInsightMetrics.CreatePrometheusRegistryAndMetricsGauge(settings.Name)

	metricsList := prometheusCommon.NewMetricsList()
	metricsList.SetCache(metricsCache)

	cacheKey := fmt.Sprintf("probeMetricsResourceHandler::%x", sha256.Sum256([]byte(r.URL.String())))
	loadedFromCache := false
	if settings.Cache != nil {
		loadedFromCache = metricsList.LoadFromCache(cacheKey)
	}

	if !loadedFromCache {
		w.Header().Add("X-metrics-cached", "false")
		for _, target := range settings.Target {
			result, err := azureInsightMetrics.FetchMetrics(ctx, subscription, target, settings)

			resourceLogger := contextLogger.WithFields(log.Fields{
				"azureSubscription": subscription,
				"azureResource":     target,
			})

			if err != nil {
				resourceLogger.Warningln(err)
				prometheusMetricRequests.With(prometheus.Labels{
					"subscriptionID": subscription,
					"handler":        ProbeMetricsResourceUrl,
					"filter":         "",
					"result":         "error",
				}).Inc()
				http.Error(w, err.Error(), http.StatusBadRequest)
				continue
			}

			resourceLogger.Debugf("fetched metrics for %v", target)
			prometheusMetricRequests.With(prometheus.Labels{
				"subscriptionID": subscription,
				"handler":        ProbeMetricsResourceUrl,
				"filter":         "",
				"result":         "success",
			}).Inc()
			result.SetGauge(metricsList, settings)
		}

		// enable caching if enabled
		if cacheDuration := settings.CacheDuration(startTime); cacheDuration != nil {
			metricsList.StoreToCache(cacheKey, *cacheDuration)
			w.Header().Add("X-metrics-cached-until", time.Now().Add(*cacheDuration).Format(time.RFC3339))
		}
	} else {
		w.Header().Add("X-metrics-cached", "true")
		prometheusMetricRequests.With(prometheus.Labels{
			"subscriptionID": "",
			"handler":        ProbeMetricsResourceUrl,
			"filter":         settings.Filter,
			"result":         "cached",
		}).Inc()
	}

	metricsList.GaugeSet(metricGauge)

	// global stats counter
	prometheusCollectTime.With(prometheus.Labels{
		"subscriptionID": subscription,
		"handler":        ProbeMetricsResourceUrl,
		"filter":         "",
	}).Observe(time.Until(startTime).Seconds())

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
