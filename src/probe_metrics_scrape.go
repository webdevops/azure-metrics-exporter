package main

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"sync"
	"time"
)

func probeMetricsScrapeHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	var err error
	var timeoutSeconds float64
	var subscription, filter, metricTagName, aggregationTagName string
	params := r.URL.Query()

	startTime := time.Now()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, PROBE_METRICS_SCRAPE_TIMEOUT_DEFAULT)
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

	if filter, err = paramsGetRequired(params, "filter"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	timespan := paramsGetWithDefault(params, "timespan", "PT1M")

	var interval *string
	if val := params.Get("interval"); val != "" {
		interval = &val
	}

	if metricTagName, err = paramsGetRequired(params, "metricTagName"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if aggregationTagName, err = paramsGetRequired(params, "aggregationTagName"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

			if metric, ok := val.Tags[metricTagName]; ok {
				if aggregation, ok := val.Tags[aggregationTagName]; ok {
					result, err := azureInsightMetrics.FetchMetrics(ctx, subscription, *val.ID, timespan, interval, *metric, *aggregation)

					if err == nil {
						Logger.Verbosef("subscription[%v] fetched auto metrics for %v", subscription, *val.ID)
						result.SetGauge(metricGauge)
					} else {
						Logger.Error(err)
					}
				}
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
		"handler":        "/probe/list",
		"filter":         filter,
	}).Observe(time.Now().Sub(startTime).Seconds())

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
