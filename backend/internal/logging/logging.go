package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Init reads log configuration from environment variables and sets the
// default slog logger. Call this once at startup before any other code
// uses slog.
//
// Environment variables:
//
//	LOG_LEVEL      - debug, info, warn, error (default: info)
//	LOG_FORMAT     - text, json  (default: text)
//	LOG_ADD_SOURCE - true, false (default: false)
func Init() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	addSource := strings.EqualFold(os.Getenv("LOG_ADD_SOURCE"), "true")

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
	}

	var handler slog.Handler
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// Component returns a logger annotated with the given component name.
func Component(name string) *slog.Logger {
	return slog.Default().With("component", name)
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
