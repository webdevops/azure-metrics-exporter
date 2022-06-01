package main

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	stringsCommon "github.com/webdevops/go-common/strings"
)

func buildContextLoggerFromRequest(r *http.Request) *log.Entry {
	logFields := log.Fields{
		"requestPath": r.URL.Path,
	}

	for name, value := range r.URL.Query() {
		fieldName := fmt.Sprintf("param%s", stringsCommon.UppercaseFirst(name))
		fieldValue := value
		logFields[fieldName] = fieldValue
	}

	return log.WithFields(logFields)
}
