package main

import (
	"context"
	"crypto/sha1" // #nosec G505
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/webdevops/azure-metrics-exporter/config"
	"github.com/webdevops/azure-metrics-exporter/metrics"

	"go.uber.org/zap"
)

func probeMetricsScrapeHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64
	var metricTagName, aggregationTagName string

	startTime := time.Now()
	contextLogger := buildContextLoggerFromRequest(r)
	registry := prometheus.NewRegistry()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, config.ProbeMetricsScrapeTimeoutDefault)
	if err != nil {
		contextLogger.Warnln(err)
		http.Error(w, fmt.Sprintf("failed to parse timeout from Prometheus header: %s", err), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	var settings metrics.RequestMetricSettings
	if settings, err = metrics.NewRequestMetricSettingsForAzureResourceApi(r, Opts); err != nil {
		contextLogger.Warnln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err = paramsGetListRequired(r.URL.Query(), "subscription"); err != nil {
		contextLogger.Warnln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if metricTagName, err = paramsGetRequired(r.URL.Query(), "metricTagName"); err != nil {
		contextLogger.Warnln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if aggregationTagName, err = paramsGetRequired(r.URL.Query(), "aggregationTagName"); err != nil {
		contextLogger.Warnln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	prober := metrics.NewMetricProber(ctx, contextLogger, w, &settings, Opts)
	prober.SetUserAgent(UserAgent + gitTag)
	prober.SetAzureClient(AzureClient)
	prober.SetAzureResourceTagManager(AzureResourceTagManager)
	prober.SetPrometheusRegistry(registry)
	if settings.Cache != nil {
		cacheKey := fmt.Sprintf("scrape:%x", sha1.Sum([]byte(r.URL.String()))) // #nosec G401
		prober.EnableMetricsCache(metricsCache, cacheKey, settings.CacheDuration(startTime))
	}

	if Opts.Azure.ServiceDiscovery.CacheDuration.Seconds() > 0 {
		prober.EnableServiceDiscoveryCache(azureCache, Opts.Azure.ServiceDiscovery.CacheDuration)
	}

	if !prober.FetchFromCache() {
		for _, subscription := range settings.Subscriptions {
			prober.ServiceDiscovery.FindSubscriptionResourcesWithScrapeTags(ctx, subscription, settings.Filter, metricTagName, aggregationTagName)
		}

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
		zap.String("method", r.Method),
		zap.Int("status", http.StatusOK),
		zap.String("latency", latency.String()),
	).Info("Request handled for /probe/metrics/scrape")
}
