package logger

import "context"

// Noop returns a Logger that discards every entry. Intended for tests and
// wiring sites where a real logger is not yet available.
func Noop() Logger { return noopLogger{} }

type noopLogger struct{}

func (noopLogger) Debug(context.Context, string, ...Field) {}
func (noopLogger) Info(context.Context, string, ...Field)  {}
func (noopLogger) Warn(context.Context, string, ...Field)  {}
func (noopLogger) Error(context.Context, string, ...Field) {}
func (noopLogger) Fatal(context.Context, string, ...Field) {}
func (n noopLogger) With(...Field) Logger                  { return n }
func (n noopLogger) WithError(error) Logger                { return n }
