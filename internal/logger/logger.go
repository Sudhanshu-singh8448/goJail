package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Init initializes the global structured JSON logger.
// Logs are one JSON line per entry for structured request logging.
func Init() *slog.Logger {
	debugEnv := strings.ToLower(os.Getenv("DEBUG"))
	debug := debugEnv == "true" || debugEnv == "1"

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
