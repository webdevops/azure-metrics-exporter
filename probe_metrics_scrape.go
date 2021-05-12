package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

func probeMetricsScrapeHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64
	var metricTagName, aggregationTagName string
	wg := sizedwaitgroup.New(opts.Prober.ConcurrencySubscription)
	params := r.URL.Query()

	startTime := time.Now()

	contextLogger := buildContextLoggerFromRequest(r)

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, ProbeMetricsScrapeTimeoutDefault)
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

	registry := prometheus.NewRegistry()
	metricList := NewMetricList()

	azureInsightMetrics := NewAzureInsightMetrics(AzureAuthorizer, registry)

	if metricTagName, err = paramsGetRequired(params, "metricTagName"); err != nil {
		contextLogger.Errorln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if aggregationTagName, err = paramsGetRequired(params, "aggregationTagName"); err != nil {
		contextLogger.Errorln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cacheKey := fmt.Sprintf("scrape::%x", sha256.Sum256([]byte(r.URL.String())))
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
			for _, subscription := range settings.Subscriptions {
				wg.Add()
				go func(subscription string) {
					defer wg.Done()
					wgResource := sizedwaitgroup.New(opts.Prober.ConcurrencySubscriptionResource)

					list, err := azureInsightMetrics.ListResources(ctx, contextLogger, subscription, settings.Filter)
					if err != nil {
						contextLogger.Errorln(err)
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}

					for _, row := range list {
						val := row

						resourceLogger := contextLogger.WithFields(log.Fields{
							"azureSubscription": subscription,
							"azureResource":     *val.ID,
						})

						wgResource.Add()
						go func() {
							defer wgResource.Done()

							if metric, ok := val.Tags[metricTagName]; ok && metric != nil {
								if aggregation, ok := val.Tags[aggregationTagName]; ok && aggregation != nil {
									settings.SetMetrics(*metric)
									settings.SetAggregations(*aggregation)

									result, err := azureInsightMetrics.FetchMetrics(ctx, subscription, *val.ID, settings)

									if err == nil {
										resourceLogger.Debugf("fetched metrics for %v", *val.ID)
										prometheusMetricRequests.With(prometheus.Labels{
											"subscriptionID": subscription,
											"handler":        ProbeMetricsScrapeUrl,
											"filter":         settings.Filter,
											"result":         "success",
										}).Inc()

										result.SendMetricToChannel(settings, metricChannel)
									} else {
										resourceLogger.Warningln(err)
										prometheusMetricRequests.With(prometheus.Labels{
											"subscriptionID": subscription,
											"handler":        ProbeMetricsScrapeUrl,
											"filter":         settings.Filter,
											"result":         "error",
										}).Inc()
									}
								}
							}
						}()
					}

					wgResource.Wait()

					// global stats counter
					prometheusCollectTime.With(prometheus.Labels{
						"subscriptionID": subscription,
						"handler":        ProbeMetricsScrapeUrl,
						"filter":         settings.Filter,
					}).Observe(time.Until(startTime).Seconds())
				}(subscription)
			}
			wg.Wait()
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
			"handler":        ProbeMetricsScrapeUrl,
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

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
