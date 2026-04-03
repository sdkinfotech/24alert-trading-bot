package logging

import (
	"context"
	"os"
	"testing"
)

func TestNewLogger_Levels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "unknown_defaults_to_info"}

	for _, lvl := range levels {
		t.Run(lvl, func(t *testing.T) {
			logger, err := NewLogger(lvl, "text", "stdout", "")
			if err != nil {
				t.Fatalf("NewLogger(%q) error: %v", lvl, err)
			}
			if logger == nil {
				t.Fatal("NewLogger() returned nil")
			}
		})
	}
}

func TestNewLogger_JSONFormat(t *testing.T) {
	logger, err := NewLogger("info", "json", "stdout", "")
	if err != nil {
		t.Fatalf("NewLogger(json) error: %v", err)
	}
	if logger == nil {
		t.Fatal("NewLogger(json) returned nil")
	}
}

func TestNewLogger_FileOutput(t *testing.T) {
	tmp, err := os.CreateTemp("", "logger-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	logger, err := NewLogger("info", "text", "file", tmp.Name())
	if err != nil {
		t.Fatalf("NewLogger(file) error: %v", err)
	}
	if logger == nil {
		t.Fatal("NewLogger(file) returned nil")
	}
}

func TestNewLogger_BothOutput(t *testing.T) {
	tmp, err := os.CreateTemp("", "logger-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	logger, err := NewLogger("info", "text", "both", tmp.Name())
	if err != nil {
		t.Fatalf("NewLogger(both) error: %v", err)
	}
	if logger == nil {
		t.Fatal("NewLogger(both) returned nil")
	}
}

func TestNewLogger_BadFilePath(t *testing.T) {
	_, err := NewLogger("info", "text", "file", "/nonexistent/dir/file.log")
	if err == nil {
		t.Error("NewLogger() with bad file path should return error")
	}
}

func TestCorrelationID_RoundTrip(t *testing.T) {
	ctx := context.Background()
	id := "test-correlation-123"

	ctx = WithCorrelationID(ctx, id)
	got := GetCorrelationID(ctx)
	if got != id {
		t.Errorf("GetCorrelationID() = %q, want %q", got, id)
	}
}

func TestGetCorrelationID_EmptyContext(t *testing.T) {
	ctx := context.Background()
	got := GetCorrelationID(ctx)
	if got != "" {
		t.Errorf("GetCorrelationID(empty ctx) = %q, want empty string", got)
	}
}

func TestFromContext(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("FromContext() returned nil")
	}
}

func TestLogger_WithContext(t *testing.T) {
	logger, err := NewLogger("info", "text", "stdout", "")
	if err != nil {
		t.Fatal(err)
	}

	ctx := WithCorrelationID(context.Background(), "abc-def")
	enriched := logger.WithContext(ctx)
	if enriched == nil {
		t.Fatal("WithContext() returned nil")
	}
}

func TestLogger_WithContext_NoCorrelation(t *testing.T) {
	logger, err := NewLogger("info", "text", "stdout", "")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	enriched := logger.WithContext(ctx)
	if enriched != logger {
		t.Error("WithContext() without correlation ID should return same logger")
	}
}
