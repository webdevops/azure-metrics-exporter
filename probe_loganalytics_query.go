package main

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/operationalinsights/v1/operationalinsights"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

func probeLogAnalyticsQueryHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64
	var workspace, query, timespan string
	params := r.URL.Query()

	startTime := time.Now()

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, PROBE_LOGANALYTICS_SCRAPE_TIMEOUT_DEFAULT)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	if workspace, err = paramsGetRequired(params, "workspace"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if query, err = paramsGetRequired(params, "query"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if timespan, err = paramsGetRequired(params, "timespan"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	queryBody := operationalinsights.QueryBody{
		Query:    &query,
		Timespan: &timespan,
	}

	result, err := azureLogAnalyticsMetrics.Query(ctx, workspace, queryBody)

	if err != nil {
		Logger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	registry := prometheus.NewRegistry()

	queryInfoGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "azurerm_loganalytics_query_result",
		Help: "Azure LogAnalytics query result",
	}, []string{})
	registry.MustRegister(queryInfoGauge)

	queryInfoGauge.With(prometheus.Labels{}).Set(boolToFloat64(result.Result != nil))

	if result.Result != nil {
		metricLabels := []string{"table"}

		if result.Result.Tables != nil {
			for _, table := range *result.Result.Tables {
				for _, column := range *table.Columns {
					metricLabels = append(metricLabels, *column.Name)
				}
			}
		}

		logRowGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "azurerm_loganalytics_query_row",
			Help: "Azure LogAnalytics query row",
		}, metricLabels)
		registry.MustRegister(logRowGauge)

		if result.Result.Tables != nil {
			for _, table := range *result.Result.Tables {
				fmt.Println(*table.Name)
				for _, row := range *table.Rows {
					rowLabels := prometheus.Labels{}

					for _, metricLabelName := range metricLabels {
						rowLabels[metricLabelName] = ""
					}
					rowLabels["table"] = *table.Name

					for colId, column := range *table.Columns {
						labelValue := ""
						cellValue := row[colId]
						if val, ok := cellValue.(bool); ok {
							labelValue = boolToString(val)
						}

						if val, ok := cellValue.(int); ok {
							labelValue = intToString(val)
						}

						if val, ok := cellValue.(int64); ok {
							labelValue = int64ToString(val)
						}

						if val, ok := cellValue.(float32); ok {
							labelValue = float32ToString(val)
						}

						if val, ok := cellValue.(float64); ok {
							labelValue = float64ToString(val)
						}

						if val, ok := cellValue.(string); ok {
							labelValue = val
						}

						rowLabels[*column.Name] = labelValue
					}

					logRowGauge.With(rowLabels).Set(1)
				}
			}
		}
	}

	// global stats counter
	prometheusCollectTime.With(prometheus.Labels{
		"subscriptionID": "",
		"handler":        PROBE_LOGANALYTICS_SCRAPE_URL,
		"filter":         query,
	}).Observe(time.Now().Sub(startTime).Seconds())

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
