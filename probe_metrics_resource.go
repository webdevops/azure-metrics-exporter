package main

import (
	"context"
	"crypto/sha1" // #nosec G505
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/webdevops/azure-metrics-exporter/config"
	"github.com/webdevops/azure-metrics-exporter/metrics"
)

func probeMetricsResourceHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64

	startTime := time.Now()
	contextLogger := buildContextLoggerFromRequest(r)
	registry := prometheus.NewRegistry()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, config.ProbeMetricsResourceTimeoutDefault)
	if err != nil {
		contextLogger.Warn(err.Error())
		http.Error(w, fmt.Sprintf("failed to parse timeout from Prometheus header: %s", err), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	var settings metrics.RequestMetricSettings
	if settings, err = metrics.NewRequestMetricSettingsForAzureResourceApi(r, Opts); err != nil {
		contextLogger.Warn(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err = paramsGetListRequired(r.URL.Query(), "subscription"); err != nil {
		contextLogger.Warn(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	prober := metrics.NewMetricProber(ctx, contextLogger.Logger, w, &settings, Opts)
	prober.SetUserAgent(UserAgent + gitTag)
	prober.SetAzureClient(AzureClient)
	prober.SetAzureResourceTagManager(AzureResourceTagManager)
	prober.SetPrometheusRegistry(registry)
	if settings.Cache != nil {
		cacheKey := fmt.Sprintf("resource:%x", sha1.Sum([]byte(r.URL.String()))) // #nosec G401
		prober.EnableMetricsCache(metricsCache, cacheKey, settings.CacheDuration(startTime))
	}

	if resourceList, err := paramsGetListRequired(r.URL.Query(), "target"); err == nil {
		targetList := []metrics.MetricProbeTarget{}
		for _, resourceId := range resourceList {
			targetList = append(
				targetList,
				metrics.MetricProbeTarget{
					ResourceId:   resourceId,
					Metrics:      settings.Metrics,
					Aggregations: settings.Aggregations,
				},
			)
		}
		prober.AddTarget(targetList...)
	} else {
		contextLogger.Error(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !prober.FetchFromCache() {
		prober.RegisterSubscriptionCollectFinishCallback(func(subscriptionId string) {
			// global stats counter
			prometheusCollectTime.With(prometheus.Labels{
				"subscriptionID": subscriptionId,
				"handler":        config.ProbeMetricsListUrl,
				"filter":         settings.Filter,
			}).Observe(time.Since(startTime).Seconds())
		})

		prober.Run()
	} else {
		w.Header().Add("X-metrics-cached", "true")
		for _, subscriptionId := range settings.Subscriptions {
			prometheusMetricRequests.With(prometheus.Labels{
				"subscriptionID": subscriptionId,
				"handler":        config.ProbeMetricsListUrl,
				"filter":         settings.Filter,
				"result":         "cached",
			}).Inc()
		}
	}

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)

	latency := time.Since(startTime)
	contextLogger.With(
		slog.String("method", r.Method),
		slog.Int("status", http.StatusOK),
		slog.Duration("latency", latency),
	).Info("request handled")
}
