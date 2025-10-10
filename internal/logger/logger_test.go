package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestInit(t *testing.T) {
	t.Run("default warn level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		Init(Config{
			Verbose: false,
			Quiet:   false,
			JSON:    true,
			Writer:  buf,
		})

		if GetLevel() != zerolog.WarnLevel {
			t.Errorf("Expected warn level, got %v", GetLevel())
		}
	})

	t.Run("debug level with verbose", func(t *testing.T) {
		buf := &bytes.Buffer{}
		Init(Config{
			Verbose: true,
			Quiet:   false,
			JSON:    true,
			Writer:  buf,
		})

		if GetLevel() != zerolog.DebugLevel {
			t.Errorf("Expected debug level, got %v", GetLevel())
		}
	})

	t.Run("error level with quiet", func(t *testing.T) {
		buf := &bytes.Buffer{}
		Init(Config{
			Verbose: false,
			Quiet:   true,
			JSON:    true,
			Writer:  buf,
		})

		if GetLevel() != zerolog.ErrorLevel {
			t.Errorf("Expected error level, got %v", GetLevel())
		}
	})

	t.Run("quiet takes precedence over verbose", func(t *testing.T) {
		buf := &bytes.Buffer{}
		Init(Config{
			Verbose: true,
			Quiet:   true,
			JSON:    true,
			Writer:  buf,
		})

		if GetLevel() != zerolog.ErrorLevel {
			t.Errorf("Expected error level (quiet precedence), got %v", GetLevel())
		}
	})
}

func TestJSONOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	Init(Config{
		Verbose: true, // Enable verbose to test info level
		Quiet:   false,
		JSON:    true,
		Writer:  buf,
	})

	Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, `"level":"info"`) {
		t.Errorf("Expected JSON output with level field, got: %s", output)
	}
	if !strings.Contains(output, `"message":"test message"`) {
		t.Errorf("Expected JSON output with message field, got: %s", output)
	}

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Errorf("Expected valid JSON output, got error: %v", err)
	}
}

func TestConsoleOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	Init(Config{
		Verbose: true, // Enable verbose to test info level
		Quiet:   false,
		JSON:    false,
		Writer:  buf,
	})

	Info().Msg("test message")

	output := buf.String()
	// Console output should contain the message but not be JSON
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected console output to contain message, got: %s", output)
	}
	// Should not be valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err == nil {
		t.Error("Expected non-JSON console output, but got valid JSON")
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func() *zerolog.Event
		level    string
		minLevel zerolog.Level
	}{
		{
			name:     "debug level",
			logFunc:  Debug,
			level:    "debug",
			minLevel: zerolog.DebugLevel,
		},
		{
			name:     "info level",
			logFunc:  Info,
			level:    "info",
			minLevel: zerolog.InfoLevel,
		},
		{
			name:     "warn level",
			logFunc:  Warn,
			level:    "warn",
			minLevel: zerolog.WarnLevel,
		},
		{
			name:     "error level",
			logFunc:  Error,
			level:    "error",
			minLevel: zerolog.ErrorLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			Init(Config{
				Verbose: true, // Enable debug to test all levels
				Quiet:   false,
				JSON:    true,
				Writer:  buf,
			})

			// Set to specific level to test filtering
			SetLevel(tt.minLevel)

			tt.logFunc().Msg("test message")

			output := buf.String()
			if !strings.Contains(output, tt.level) {
				t.Errorf("Expected %s level in output, got: %s", tt.level, output)
			}
		})
	}
}

func TestLogFiltering(t *testing.T) {
	t.Run("debug and info messages filtered at warn level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		Init(Config{
			Verbose: false, // Warn level (default)
			Quiet:   false,
			JSON:    true,
			Writer:  buf,
		})

		Debug().Msg("debug message")
		Info().Msg("info message")
		output := buf.String()

		if output != "" {
			t.Errorf("Expected no output for debug/info at warn level, got: %s", output)
		}
	})

	t.Run("info messages filtered at error level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		Init(Config{
			Verbose: false,
			Quiet:   true, // Error level
			JSON:    true,
			Writer:  buf,
		})

		Info().Msg("info message")
		output := buf.String()

		if output != "" {
			t.Errorf("Expected no output for info at error level, got: %s", output)
		}
	})

	t.Run("warnings logged at warn level (default)", func(t *testing.T) {
		buf := &bytes.Buffer{}
		Init(Config{
			Verbose: false, // Warn level (default)
			Quiet:   false,
			JSON:    true,
			Writer:  buf,
		})

		Warn().Msg("warning message")
		output := buf.String()

		if !strings.Contains(output, "warning message") {
			t.Errorf("Expected warning message to be logged at warn level, got: %s", output)
		}
	})

	t.Run("error messages logged at error level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		Init(Config{
			Verbose: false,
			Quiet:   true, // Error level
			JSON:    true,
			Writer:  buf,
		})

		Error().Msg("error message")
		output := buf.String()

		if !strings.Contains(output, "error message") {
			t.Errorf("Expected error message to be logged at error level, got: %s", output)
		}
	})
}

func TestWithContext(t *testing.T) {
	buf := &bytes.Buffer{}
	Init(Config{
		Verbose: true, // Enable verbose to test info level
		Quiet:   false,
		JSON:    true,
		Writer:  buf,
	})

	fields := map[string]interface{}{
		"user":   "testuser",
		"action": "test_action",
	}

	contextLogger := WithContext(fields)
	contextLogger.Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, `"user":"testuser"`) {
		t.Errorf("Expected user field in output, got: %s", output)
	}
	if !strings.Contains(output, `"action":"test_action"`) {
		t.Errorf("Expected action field in output, got: %s", output)
	}
}

func TestWithCommand(t *testing.T) {
	buf := &bytes.Buffer{}
	Init(Config{
		Verbose: true, // Enable verbose to test info level
		Quiet:   false,
		JSON:    true,
		Writer:  buf,
	})

	commandLogger := WithCommand("diff")
	commandLogger.Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, `"command":"diff"`) {
		t.Errorf("Expected command field in output, got: %s", output)
	}
}

func TestWithRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	Init(Config{
		Verbose: true, // Enable verbose to test info level
		Quiet:   false,
		JSON:    true,
		Writer:  buf,
	})

	requestLogger := WithRequestID("req-123")
	requestLogger.Info().Msg("test message")

	output := buf.String()
	if !strings.Contains(output, `"request_id":"req-123"`) {
		t.Errorf("Expected request_id field in output, got: %s", output)
	}
}

func TestGet(t *testing.T) {
	buf := &bytes.Buffer{}
	Init(Config{
		Verbose: true, // Enable verbose to test info level
		Quiet:   false,
		JSON:    true,
		Writer:  buf,
	})

	logger := Get()
	if logger == nil {
		t.Error("Expected Get() to return non-nil logger")
	}

	// Verify we can use the returned logger
	logger.Info().Msg("test message")
	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message from Get() logger, got: %s", output)
	}
}

func TestSetLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	Init(Config{
		Verbose: true, // Start at debug
		Quiet:   false,
		JSON:    true,
		Writer:  buf,
	})

	if GetLevel() != zerolog.DebugLevel {
		t.Errorf("Expected initial debug level, got %v", GetLevel())
	}

	// Change to warn level
	SetLevel(zerolog.WarnLevel)
	if GetLevel() != zerolog.WarnLevel {
		t.Errorf("Expected warn level after SetLevel, got %v", GetLevel())
	}

	// Info should be filtered now
	Info().Msg("info message")
	if buf.String() != "" {
		t.Errorf("Expected no output for info at warn level, got: %s", buf.String())
	}

	// Warn should pass through
	buf.Reset()
	Warn().Msg("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Errorf("Expected warn message to be logged, got: %s", buf.String())
	}
}
