// Package logger provides a Zap-backed structured logger with automatic
// OpenTelemetry trace-ID correlation. Log entries are tagged with
// Datadog-compatible fields so Datadog's UI links logs to APM traces.
//
// The package is service-agnostic: it imposes no RequestContext or similar
// type. Services wrap the returned Logger with their own helpers when they
// want to inject per-request fields.
package logger

import "context"

// Logger is the structured logging contract shared across Jinko services.
// All methods are context-aware so trace/span IDs can be attached for
// log-to-trace correlation.
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...Field)
	Info(ctx context.Context, msg string, fields ...Field)
	Warn(ctx context.Context, msg string, fields ...Field)
	Error(ctx context.Context, msg string, fields ...Field)
	Fatal(ctx context.Context, msg string, fields ...Field)

	// With returns a derived logger with the given fields pre-populated on
	// every subsequent log entry.
	With(fields ...Field) Logger

	// WithError is shorthand for With(Error(err)).
	WithError(err error) Logger
}

// Config controls logger construction.
type Config struct {
	// Level is one of "debug", "info", "warn", "error", "fatal". Defaults
	// to "info" when empty.
	Level string

	// Format is "json" (production) or "console" (development). Defaults
	// to "json" when empty.
	Format string

	// Output is "stdout" (default) or "stderr".
	Output string
}

// DefaultConfig returns production-appropriate defaults: info level, JSON
// format, stdout output.
func DefaultConfig() Config {
	return Config{Level: "info", Format: "json", Output: "stdout"}
}

// New builds a Logger from the given Config. It returns an error if the
// level or format are unrecognized.
func New(cfg Config) (Logger, error) {
	return newZapLogger(cfg)
}

// MustNew panics on construction failure. Intended for main() where a
// logger must always be available.
func MustNew(cfg Config) Logger {
	l, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return l
}
