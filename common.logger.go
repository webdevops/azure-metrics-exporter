package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/webdevops/go-common/log/slogger"
)

var (
	logger      *slogger.Logger
	LoggerLevel = new(slog.LevelVar)
)

func initLogger() *slogger.Logger {
	ShowSource := Opts.Logger.Source != "off"

	loggerOpts := slogger.NewHandlerOptions(&slog.HandlerOptions{
		AddSource: ShowSource,
		Level:     LoggerLevel,
	})

	if Opts.Logger.Source != "off" {
		loggerOpts.SourceMode = slogger.SourceMode(Opts.Logger.Source)
	}

	loggerOpts.ShowTime = Opts.Logger.Time

	switch strings.ToLower(Opts.Logger.Format) {
	case "text":
		logger = slogger.New(slog.NewTextHandler(os.Stdout, loggerOpts.HandlerOptions))
	case "json":
		logger = slogger.New(slog.NewJSONHandler(os.Stderr, loggerOpts.HandlerOptions))
	default:
		fmt.Println("Unknown log format:", Opts.Logger.Format)
		os.Exit(1)
	}

	switch strings.ToLower(Opts.Logger.Level) {
	case "trace":
		LoggerLevel.Set(slogger.LevelTrace)
	case "debug":
		LoggerLevel.Set(slog.LevelDebug)
	case "info":
		LoggerLevel.Set(slog.LevelInfo)
	case "warning":
		LoggerLevel.Set(slog.LevelWarn)
	case "error":
		LoggerLevel.Set(slog.LevelError)
	default:
		fmt.Println("Unknown log level:", Opts.Logger.Level)
		os.Exit(1)
	}

	slog.SetDefault(logger.Logger)

	return logger
}
