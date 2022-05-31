package main

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
)

func buildContextLoggerFromRequest(r *http.Request) *log.Entry {
	logFields := log.Fields{
		"requestPath": r.URL.Path,
	}

	for name, value := range r.URL.Query() {
		fieldName := fmt.Sprintf("param%s", prometheusCommon.StringToTitle(name))
		fieldValue := value
		logFields[fieldName] = fieldValue
	}

	return log.WithFields(logFields)
}
