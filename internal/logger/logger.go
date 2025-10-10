package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	// globalLogger is the application-wide logger instance
	globalLogger zerolog.Logger
)

// Config holds logger configuration
type Config struct {
	Verbosity int       // Verbosity level: 0=warn, 1=info, 2=debug, 3+=trace
	Quiet     bool      // Only error level logging
	JSON      bool      // Output in JSON format
	Writer    io.Writer // Output writer (defaults to os.Stderr)
}

// Init initializes the global logger with the provided configuration
func Init(cfg Config) {
	// Set default writer if not provided
	if cfg.Writer == nil {
		cfg.Writer = os.Stderr
	}

	// Determine log level based on flags
	// Default (0):    Warn level  (only warnings and errors)
	// Verbosity 1:    Info level  (info, warnings, and errors)
	// Verbosity 2:    Debug level (debug, info, warnings, and errors)
	// Verbosity 3+:   Trace level (trace, debug, info, warnings, and errors)
	// Quiet:          Error level (only errors)
	level := zerolog.WarnLevel
	if cfg.Quiet {
		level = zerolog.ErrorLevel
	} else {
		switch cfg.Verbosity {
		case 0:
			level = zerolog.WarnLevel
		case 1:
			level = zerolog.InfoLevel
		case 2:
			level = zerolog.DebugLevel
		default: // 3 or higher
			level = zerolog.TraceLevel
		}
	}

	// Configure output format
	var output io.Writer
	if cfg.JSON {
		// JSON output - write directly to writer
		output = cfg.Writer
	} else {
		// Console output with pretty formatting
		output = zerolog.ConsoleWriter{
			Out:        cfg.Writer,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	// Create logger
	globalLogger = zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Logger()

	// Update global log
	log.Logger = globalLogger
}

// Get returns the global logger instance
func Get() *zerolog.Logger {
	return &globalLogger
}

// Trace logs a message at trace level (most verbose)
func Trace() *zerolog.Event {
	return globalLogger.Trace()
}

// Debug logs a message at debug level
func Debug() *zerolog.Event {
	return globalLogger.Debug()
}

// Info logs a message at info level
func Info() *zerolog.Event {
	return globalLogger.Info()
}

// Warn logs a message at warn level
func Warn() *zerolog.Event {
	return globalLogger.Warn()
}

// Error logs a message at error level
func Error() *zerolog.Event {
	return globalLogger.Error()
}

// Fatal logs a message at fatal level and exits
func Fatal() *zerolog.Event {
	return globalLogger.Fatal()
}

// WithContext returns a logger with additional context fields
func WithContext(fields map[string]interface{}) zerolog.Logger {
	ctx := globalLogger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	return ctx.Logger()
}

// WithCommand returns a logger with command context
func WithCommand(command string) zerolog.Logger {
	return globalLogger.With().Str("command", command).Logger()
}

// WithRequestID returns a logger with request ID context
func WithRequestID(requestID string) zerolog.Logger {
	return globalLogger.With().Str("request_id", requestID).Logger()
}

// SetLevel changes the global log level
func SetLevel(level zerolog.Level) {
	globalLogger = globalLogger.Level(level)
}

// GetLevel returns the current log level
func GetLevel() zerolog.Level {
	return globalLogger.GetLevel()
}
