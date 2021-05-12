package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
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

	registry := prometheus.NewRegistry()
	metricList := NewMetricList()

	azureInsightMetrics := NewAzureInsightMetrics(AzureAuthorizer, registry)

	cacheKey := fmt.Sprintf("resource::%x", sha256.Sum256([]byte(r.URL.String())))
	loadedFromCache := false
	if settings.Cache != nil {
		if val, ok := metricsCache.Get(cacheKey); ok {
			metricList = val.(*MetricList)
			loadedFromCache = true
		}
	}

	if !loadedFromCache {
		w.Header().Add("X-metrics-cached", "false")
		metricChannel := make(chan PrometheusMetricResult)

		go func() {
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

				result.SendMetricToChannel(settings, metricChannel)
			}

			close(metricChannel)
		}()

		// collect metrics from channel
		for result := range metricChannel {
			metric := MetricRow{
				Labels: result.Labels,
				Value:  result.Value,
			}
			metricList.Add(result.Name, metric)
		}

		// enable caching if enabled
		if settings.Cache != nil {
			if cacheDuration := settings.CacheDuration(startTime); cacheDuration != nil {
				_ = metricsCache.Add(cacheKey, metricList, *cacheDuration)
				w.Header().Add("X-metrics-cached-until", time.Now().Add(*cacheDuration).Format(time.RFC3339))
			}
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

	// create prometheus metrics and set rows
	for _, metricName := range metricList.GetMetricNames() {
		gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricName,
			Help: "Azure monitor insight metric",
		},
			metricList.GetMetricLabelNames(metricName),
		)
		registry.MustRegister(gauge)

		for _, row := range metricList.GetMetricList(metricName) {
			gauge.With(row.Labels).Set(row.Value)
		}
	}

	// global stats counter
	prometheusCollectTime.With(prometheus.Labels{
		"subscriptionID": subscription,
		"handler":        ProbeMetricsResourceUrl,
		"filter":         "",
	}).Observe(time.Until(startTime).Seconds())

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
