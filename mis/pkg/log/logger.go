package log

import (
	"log/slog"
	"os"
	"strings"
)

var ValidLogLevels = map[string]struct{}{
	"error": {},
	"warn":  {},
	"info":  {},
	"debug": {},
}

// Init initializes the global slog logger with the given log level.
// level: "debug", "info", "warn", "error"
func New(level string) *slog.Logger {
	var logLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	return slog.New(handler)
}
