package logging

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

var sensitiveLogKey = regexp.MustCompile(`(?i)(api[_-]?key|authorization|password|passcode|secret|token|dsn)`)

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
	secrets := sensitiveEnvironmentValues()
	opts.ReplaceAttr = func(_ []string, attr slog.Attr) slog.Attr {
		if sensitiveLogKey.MatchString(attr.Key) {
			return slog.String(attr.Key, "[REDACTED]")
		}
		switch attr.Value.Kind() {
		case slog.KindString:
			return slog.String(attr.Key, RedactText(attr.Value.String(), secrets...))
		case slog.KindAny:
			return slog.String(attr.Key, RedactText(fmt.Sprint(attr.Value.Any()), secrets...))
		default:
			return attr
		}
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

// RedactText removes known credentials from log messages and error values.
func RedactText(value string, secrets ...string) string {
	redacted := value
	for _, secret := range secrets {
		if secret != "" {
			redacted = strings.ReplaceAll(redacted, secret, "[REDACTED]")
		}
	}
	return redacted
}

func sensitiveEnvironmentValues() []string {
	values := make([]string, 0)
	for _, entry := range os.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if found && len(value) >= 4 && sensitiveLogKey.MatchString(name) {
			values = append(values, value)
		}
	}
	return values
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
