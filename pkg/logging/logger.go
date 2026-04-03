package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

type ctxKey struct{}

// NewLogger creates a new logger with specified configuration
func NewLogger(level, format, output string, filePath string) (*Logger, error) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var out io.Writer
	switch output {
	case "file":
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		out = f
	case "both":
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		out = io.MultiWriter(os.Stdout, f)
	default:
		out = os.Stdout
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(out, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewTextHandler(out, &slog.HandlerOptions{Level: logLevel})
	}

	return &Logger{Logger: slog.New(handler)}, nil
}

// Logger wraps slog.Logger with convenience methods
type Logger struct {
	*slog.Logger
}

// WithCorrelationID adds a correlation ID to ctx
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// GetCorrelationID extracts correlation ID from ctx
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}

// FromContext returns the logger stored in ctx or the default logger
func FromContext(ctx context.Context) *Logger {
	return &Logger{Logger: slog.Default()}
}

// WithContext enriches the logger with context values (correlation ID, etc.)
func (l *Logger) WithContext(ctx context.Context) *Logger {
	if id := GetCorrelationID(ctx); id != "" {
		return &Logger{Logger: l.Logger.With("correlation_id", id)}
	}
	return l
}
