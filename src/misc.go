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
