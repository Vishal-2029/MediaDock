package utils

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger creates a structured slog.Logger configured for JSON output in production
// and readable text or JSON in development.
func NewLogger(appEnv string) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(appEnv, "development") || strings.EqualFold(appEnv, "debug") {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(appEnv, "production") || strings.EqualFold(appEnv, "staging") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

