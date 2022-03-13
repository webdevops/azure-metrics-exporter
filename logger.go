package main

import (
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

func buildContextLoggerFromRequest(r *http.Request) *log.Entry {
	logFields := log.Fields{
		"requestPath": r.URL.Path,
	}

	for name, value := range r.URL.Query() {
		fieldName := fmt.Sprintf("param%s", strings.Title(name))
		fieldValue := value
		logFields[fieldName] = fieldValue
	}

	return log.WithFields(logFields)
}
