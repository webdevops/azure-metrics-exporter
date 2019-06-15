package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strings"
	"sync"
	"time"
)

func probeMetricsListHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	var err error
	var timeoutSeconds float64
	var subscriptions, filter string
	params := r.URL.Query()

	startTime := time.Now()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, PROBE_METRICS_LIST_TIMEOUT_DEFAULT)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	registry, metricGauge := azureInsightMetrics.CreatePrometheusRegistryAndMetricsGauge()

	if subscriptions, err = paramsGetRequired(params, "subscription"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if filter, err = paramsGetRequired(params, "filter"); err != nil {
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

	for _, subscription := range strings.Split(subscriptions, ",") {
		subscription = strings.TrimSpace(subscription)

		go func(subscription string) {
			// fetch list of resources
			list, err := azureInsightMetrics.ListResources(subscription, filter)

			if err != nil {
				Logger.Error(err)
				http.Error(w, err.Error(), http.StatusBadRequest)
			}

			for list.NotDone() {
				val := list.Value()

				wg.Add(1)
				go func() {
					defer wg.Done()
					result, err := azureInsightMetrics.FetchMetrics(ctx, subscription, *val.ID, timespan, interval, metric, aggregation)

					if err == nil {
						Logger.Verbosef("subscription[%v] fetched auto metrics for %v", subscription, *val.ID)
						result.SetGauge(metricGauge)
					} else {
						Logger.Error(err)
					}
				}()

				if list.NextWithContext(ctx) != nil {
					break
				}
			}

			wg.Wait()

			// global stats counter
			prometheusCollectTime.With(prometheus.Labels{
				"subscriptionID": subscription,
				"handler":        PROBE_METRICS_LIST_URL,
				"filter":         filter,
			}).Observe(time.Now().Sub(startTime).Seconds())

		} (subscription)
	}

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
