package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	stringsCommon "github.com/webdevops/go-common/strings"

	"github.com/webdevops/go-common/log/slogger"
)

func buildContextLoggerFromRequest(r *http.Request) *slogger.Logger {
	contextLogger := logger.With(slog.String("requestPath", r.URL.Path))

	for name, value := range r.URL.Query() {
		fieldName := fmt.Sprintf("param%s", stringsCommon.UppercaseFirst(name))
		fieldValue := value

		contextLogger = contextLogger.With(slog.Any(fieldName, fieldValue))
	}

	return contextLogger
}

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

func paramsGetRequired(params url.Values, name string) (value string, err error) {
	value = params.Get(name)
	if value == "" {
		err = fmt.Errorf("parameter \"%v\" is missing", name)
	}

	return
}

func paramsGetList(params url.Values, name string) (list []string, err error) {
	for _, v := range params[name] {
		list = append(list, strings.Split(v, ",")...)
	}
	return
}

func paramsGetListRequired(params url.Values, name string) (list []string, err error) {
	list, err = paramsGetList(params, name)

	if len(list) == 0 {
		err = fmt.Errorf("parameter \"%v\" is missing", name)
		return
	}

	return
}
