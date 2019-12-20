package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/remeh/sizedwaitgroup"
	"net/http"
	"strings"
	"time"
)

func probeMetricsScrapeHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64
	var subscriptions, filter, metricTagName, aggregationTagName string
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

	metricName := paramsGetWithDefault(params, "name", PROMETHEUS_METRIC_NAME)
	registry, metricGauge := azureInsightMetrics.CreatePrometheusRegistryAndMetricsGauge(metricName)

	if subscriptions, err = paramsGetRequired(params, "subscription"); err != nil {
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if filter, err = paramsGetRequired(params, "filter"); err != nil {
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	timespan := paramsGetWithDefault(params, "timespan", "PT1M")

	var interval *string
	if val := params.Get("interval"); val != "" {
		interval = &val
	}

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

	for _, subscription := range strings.Split(subscriptions, ",") {
		subscription = strings.TrimSpace(subscription)

		wg.Add()
		go func(subscription string) {
			defer wg.Done()
			wgResource := sizedwaitgroup.New(opts.ConcurrencySubscriptionResource)

			list, err := azureInsightMetrics.ListResources(subscription, filter)

			if err != nil {
				Logger.Error(err)
				http.Error(w, err.Error(), http.StatusBadRequest)
			}

			for list.NotDone() {
				val := list.Value()

				wgResource.Add()
				go func() {
					defer wgResource.Done()

					if metric, ok := val.Tags[metricTagName]; ok {
						if aggregation, ok := val.Tags[aggregationTagName]; ok {
							result, err := azureInsightMetrics.FetchMetrics(ctx, subscription, *val.ID, timespan, interval, *metric, *aggregation)

							if err == nil {
								Logger.Verbosef("subscription[%v] fetched auto metrics for %v", subscription, *val.ID)
								result.SetGauge(metricGauge)
								prometheusMetricRequests.With(prometheus.Labels{
									"subscriptionID": subscription,
									"handler":        PROBE_METRICS_SCRAPE_URL,
									"filter":         filter,
									"result":         "success",
								}).Inc()
							} else {
								Logger.Error(err)
								prometheusMetricRequests.With(prometheus.Labels{
									"subscriptionID": subscription,
									"handler":        PROBE_METRICS_SCRAPE_URL,
									"filter":         filter,
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
				"filter":         filter,
			}).Observe(time.Now().Sub(startTime).Seconds())
		}(subscription)
	}

	wg.Wait()

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
