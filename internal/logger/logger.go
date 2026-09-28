package logger

import (
	"log/slog"
	"os"
	"strings"
)

// ParseLevel converts a string representation to slog.Level.
func ParseLevel(lvl string, verbose bool) slog.Level {
	if verbose {
		return slog.LevelDebug
	}
	switch strings.ToLower(strings.TrimSpace(lvl)) {
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

// Init configures and sets the default global slog logger.
func Init(levelStr string, verbose bool, isJSON bool) *slog.Logger {
	level := ParseLevel(levelStr, verbose)

	// Environment variable override if not explicitly specified via flag
	if envLvl := os.Getenv("LOG_LEVEL"); envLvl != "" && !verbose && levelStr == "" {
		level = ParseLevel(envLvl, false)
	}

	useJSON := isJSON || strings.ToLower(os.Getenv("LOG_FORMAT")) == "json"

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && !useJSON {
				// Shorter timestamp: hh:mm:ss.mmm (e.g. 19:22:07.939 instead of 2026-09-25T19:22:07.939+07:00)
				return slog.String(slog.TimeKey, a.Value.Time().Format("15:04:05.000"))
			}
			return a
		},
	}

	var handler slog.Handler
	if useJSON {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}
