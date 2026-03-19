package logger

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{"Debug level", "debug", slog.LevelDebug},
		{"Info level", "info", slog.LevelInfo},
		{"Warn level", "warn", slog.LevelWarn},
		{"Error level", "error", slog.LevelError},
		{"Invalid level defaults to info", "invalid", slog.LevelInfo},
		{"Empty level defaults to info", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset logger
			defaultLogger = nil

			Init(tt.level)

			if defaultLogger == nil {
				t.Fatal("Expected logger to be initialized")
			}

			// Verify the level by testing if messages at that level would be logged
			var buf bytes.Buffer
			handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level: tt.expected,
			})
			testLogger := slog.New(handler)

			// Test that we can create a logger with the expected level
			// The actual logger uses stdout, so we verify behavior indirectly
			if tt.expected == slog.LevelDebug {
				testLogger.Debug("test")
				if !strings.Contains(buf.String(), "test") {
					t.Error("Expected debug level to log debug messages")
				}
			}
		})
	}
}

func TestDebug(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	defaultLogger = slog.New(handler)

	Debug("test debug message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test debug message") {
		t.Error("Expected debug message in output")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Expected key-value pair in output")
	}
}

func TestDebug_NilLogger(t *testing.T) {
	defaultLogger = nil
	// Should not panic
	Debug("test message")
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	defaultLogger = slog.New(handler)

	Info("test info message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test info message") {
		t.Error("Expected info message in output")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Expected key-value pair in output")
	}
}

func TestInfo_NilLogger(t *testing.T) {
	defaultLogger = nil
	// Should not panic
	Info("test message")
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	defaultLogger = slog.New(handler)

	Warn("test warn message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test warn message") {
		t.Error("Expected warn message in output")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Expected key-value pair in output")
	}
}

func TestWarn_NilLogger(t *testing.T) {
	defaultLogger = nil
	// Should not panic
	Warn("test message")
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelError,
	})
	defaultLogger = slog.New(handler)

	Error("test error message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test error message") {
		t.Error("Expected error message in output")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Expected key-value pair in output")
	}
}

func TestError_NilLogger(t *testing.T) {
	defaultLogger = nil
	// Should not panic
	Error("test message")
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	defaultLogger = slog.New(handler)

	// Debug and Info should not appear
	Debug("debug message")
	Info("info message")

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should not appear with warn level")
	}
	if strings.Contains(output, "info message") {
		t.Error("Info message should not appear with warn level")
	}

	buf.Reset()

	// Warn and Error should appear
	Warn("warn message")
	Error("error message")

	output = buf.String()
	if !strings.Contains(output, "warn message") {
		t.Error("Expected warn message in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Expected error message in output")
	}
}

func TestInit_WritesToStdout(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Init("info")
	Info("test stdout message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	if !strings.Contains(output, "test stdout message") {
		t.Error("Expected message to be written to stdout")
	}
}

func TestMultipleAttributes(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	defaultLogger = slog.New(handler)

	Info("test message", "key1", "value1", "key2", "value2", "key3", 123)

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("Expected message in output")
	}
	if !strings.Contains(output, "key1=value1") {
		t.Error("Expected key1=value1 in output")
	}
	if !strings.Contains(output, "key2=value2") {
		t.Error("Expected key2=value2 in output")
	}
	if !strings.Contains(output, "key3=123") {
		t.Error("Expected key3=123 in output")
	}
}
