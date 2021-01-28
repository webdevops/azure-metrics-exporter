package main

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/operationalinsights/v1/operationalinsights"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"net/http"
	"time"
)

func probeLogAnalyticsQueryHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var timeoutSeconds float64
	var workspace, query, timespan string
	params := r.URL.Query()

	startTime := time.Now()

	contextLogger := buildContextLoggerFromRequest(r)

	// If a timeout is configured via the Prometheus header, add it to the request.
	timeoutSeconds, err = getPrometheusTimeout(r, ProbeLoganalyticsScrapeTimeoutDefault)
	if err != nil {
		contextLogger.Error(err)
		http.Error(w, fmt.Sprintf("failed to parse timeout from Prometheus header: %s", err), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()
	r = r.WithContext(ctx)

	if workspace, err = paramsGetRequired(params, "workspace"); err != nil {
		contextLogger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if query, err = paramsGetRequired(params, "query"); err != nil {
		contextLogger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if timespan, err = paramsGetRequired(params, "timespan"); err != nil {
		contextLogger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	queryBody := operationalinsights.QueryBody{
		Query:    &query,
		Timespan: &timespan,
	}

	result, err := azureLogAnalyticsMetrics.Query(ctx, workspace, queryBody)

	if err != nil {
		contextLogger.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	registry := prometheus.NewRegistry()

	queryInfoGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "azurerm_loganalytics_query_result",
		Help: "Azure LogAnalytics query result",
	}, []string{})
	registry.MustRegister(queryInfoGauge)

	queryInfoGauge.With(prometheus.Labels{}).Set(boolToFloat64(result.Result != nil))

	metricsList := prometheusCommon.NewMetricsList()
	metricsList.SetCache(metricsCache)

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

					metricsList.AddInfo(rowLabels)
				}
			}
		}

		metricsList.GaugeSet(logRowGauge)
	}

	// global stats counter
	prometheusCollectTime.With(prometheus.Labels{
		"subscriptionID": "",
		"handler":        ProbeLoganalyticsScrapeUrl,
		"filter":         query,
	}).Observe(time.Since(startTime).Seconds())

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
