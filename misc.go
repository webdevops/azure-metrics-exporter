package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func getPrometheusTimeout(r *http.Request, defaultTimeout float64) (timeout float64, err error) {
	// If a timeout is configured via the Prometheus header, add it to the request.
	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		timeout, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return
		}
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return
}

func paramsGetWithDefault(params url.Values, name, defaultValue string) (value string) {
	value = params.Get(name)
	if value == "" {
		value = defaultValue
	}
	return
}

func paramsGetRequired(params url.Values, name string) (value string, err error) {
	value = params.Get(name)
	if value == "" {
		err = errors.New(fmt.Sprintf("%v parameter is missing", name))
	}

	return
}

func boolToFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intToString(v int) string {
	return strconv.FormatInt(int64(v), 10)
}

func int32ToString(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func float32ToString(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', 6, 64)
}

func float64ToString(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}
