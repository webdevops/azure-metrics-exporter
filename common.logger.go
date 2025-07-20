package main

import (
	"log/slog"
	"os"

	"github.com/webdevops/go-common/log/slogger"
)

var (
	logger      *slogger.Logger
	LoggerLevel = new(slog.LevelVar)
)

func initLogger() *slogger.Logger {
	loggerOpts := slogger.NewHandlerOptions(&slog.HandlerOptions{
		AddSource: Opts.Logger.Debug,
		Level:     LoggerLevel,
	})

	loggerOpts.ShortenSourcePath = true

	if Opts.Logger.Json {
		loggerOpts.ShowTime = false
		logger = slogger.New(slog.NewJSONHandler(os.Stderr, loggerOpts.HandlerOptions))
	} else {
		logger = slogger.New(slog.NewTextHandler(os.Stdout, loggerOpts.HandlerOptions))
	}

	if Opts.Logger.Debug {
		LoggerLevel.Set(slog.LevelDebug)
	}

	slog.SetDefault(logger.Logger)

	return logger
}
