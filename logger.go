package main

import (
	"fmt"
	"net/http"

	stringsCommon "github.com/webdevops/go-common/strings"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.SugaredLogger
)

func buildContextLoggerFromRequest(r *http.Request) *zap.SugaredLogger {
	contextLogger := logger.With(zap.String("requestPath", r.URL.Path))

	for name, value := range r.URL.Query() {
		fieldName := fmt.Sprintf("param%s", stringsCommon.UppercaseFirst(name))
		fieldValue := value

		contextLogger = contextLogger.With(zap.Any(fieldName, fieldValue))
	}

	return contextLogger
}

func initLogger() *zap.SugaredLogger {
	var config zap.Config
	if opts.Logger.Development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	config.Encoding = "console"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// debug level
	if opts.Logger.Debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	// json log format
	if opts.Logger.Json {
		config.Encoding = "json"

		// if running in containers, logs already enriched with timestamp by the container runtime
		config.EncoderConfig.TimeKey = ""
	}

	// build logger
	log, err := config.Build()
	if err != nil {
		panic(err)
	}
	logger = log.Sugar()
	return logger
}
