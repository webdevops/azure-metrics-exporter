package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/remeh/sizedwaitgroup"
	"net/http"
	"time"
)

func probeMetricsScrapeHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64
	var metricTagName, aggregationTagName string
	wg := sizedwaitgroup.New(opts.ConcurrencySubscription)
	params := r.URL.Query()

	startTime := time.Now()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, PROBE_METRICS_SCRAPE_TIMEOUT_DEFAULT)
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
	registry, metricGauge := azureInsightMetrics.CreatePrometheusRegistryAndMetricsGauge(settings.Name)

	if metricTagName, err = paramsGetRequired(params, "metricTagName"); err != nil {
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if aggregationTagName, err = paramsGetRequired(params, "aggregationTagName"); err != nil {
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, subscription := range settings.Subscriptions {
		wg.Add()
		go func(subscription string) {
			defer wg.Done()
			wgResource := sizedwaitgroup.New(opts.ConcurrencySubscriptionResource)

			list, err := azureInsightMetrics.ListResources(subscription, settings.Filter)

			if err != nil {
				Logger.Error(err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			for list.NotDone() {
				val := list.Value()

				wgResource.Add()
				go func() {
					defer wgResource.Done()

					if metric, ok := val.Tags[metricTagName]; ok && metric != nil {
						if aggregation, ok := val.Tags[aggregationTagName]; ok && aggregation != nil {
							settings.Metric = *metric
							settings.Aggregation = *aggregation

							result, err := azureInsightMetrics.FetchMetrics(ctx, subscription, *val.ID, settings)

							if err == nil {
								Logger.Verbosef("subscription[%v] fetched auto metrics for %v", subscription, *val.ID)
								result.SetGauge(metricGauge, settings)
								prometheusMetricRequests.With(prometheus.Labels{
									"subscriptionID": subscription,
									"handler":        PROBE_METRICS_SCRAPE_URL,
									"filter":         settings.Filter,
									"result":         "success",
								}).Inc()
							} else {
								Logger.Warningln(err)
								prometheusMetricRequests.With(prometheus.Labels{
									"subscriptionID": subscription,
									"handler":        PROBE_METRICS_SCRAPE_URL,
									"filter":         settings.Filter,
									"result":         "error",
								}).Inc()
							}
						}
					}
				}()

				if list.NextWithContext(ctx) != nil {
					break
				}
			}

			wgResource.Wait()

			// global stats counter
			prometheusCollectTime.With(prometheus.Labels{
				"subscriptionID": subscription,
				"handler":        PROBE_METRICS_SCRAPE_URL,
				"filter":         settings.Filter,
			}).Observe(time.Now().Sub(startTime).Seconds())
		}(subscription)
	}

	wg.Wait()

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
