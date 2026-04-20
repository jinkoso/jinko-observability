package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapLogger struct {
	zl *zap.Logger
}

func newZapLogger(cfg Config) (*zapLogger, error) {
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.Format == "" {
		cfg.Format = "json"
	}
	if cfg.Output == "" {
		cfg.Output = "stdout"
	}

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	enc, err := newEncoder(cfg.Format)
	if err != nil {
		return nil, err
	}

	sink, err := newSink(cfg.Output)
	if err != nil {
		return nil, err
	}

	core := zapcore.NewCore(enc, sink, level)
	zl := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	return &zapLogger{zl: zl}, nil
}

func parseLevel(s string) (zapcore.Level, error) {
	switch s {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("logger: unknown level %q", s)
	}
}

func newEncoder(format string) (zapcore.Encoder, error) {
	ec := zap.NewProductionEncoderConfig()
	ec.TimeKey = "timestamp"
	ec.EncodeTime = zapcore.ISO8601TimeEncoder
	ec.MessageKey = "message"
	ec.LevelKey = "level"
	ec.CallerKey = "caller"
	ec.StacktraceKey = "stacktrace"

	switch format {
	case "json":
		return zapcore.NewJSONEncoder(ec), nil
	case "console":
		ec.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return zapcore.NewConsoleEncoder(ec), nil
	default:
		return nil, fmt.Errorf("logger: unknown format %q", format)
	}
}

func newSink(output string) (zapcore.WriteSyncer, error) {
	switch output {
	case "stdout":
		return zapcore.Lock(os.Stdout), nil
	case "stderr":
		return zapcore.Lock(os.Stderr), nil
	default:
		return nil, fmt.Errorf("logger: unknown output %q", output)
	}
}

func (l *zapLogger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.zl.Debug(msg, l.convert(withTrace(ctx, fields))...)
}

func (l *zapLogger) Info(ctx context.Context, msg string, fields ...Field) {
	l.zl.Info(msg, l.convert(withTrace(ctx, fields))...)
}

func (l *zapLogger) Warn(ctx context.Context, msg string, fields ...Field) {
	l.zl.Warn(msg, l.convert(withTrace(ctx, fields))...)
}

func (l *zapLogger) Error(ctx context.Context, msg string, fields ...Field) {
	l.zl.Error(msg, l.convert(withTrace(ctx, fields))...)
}

func (l *zapLogger) Fatal(ctx context.Context, msg string, fields ...Field) {
	l.zl.Fatal(msg, l.convert(withTrace(ctx, fields))...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{zl: l.zl.With(l.convert(fields)...)}
}

func (l *zapLogger) WithError(err error) Logger {
	return &zapLogger{zl: l.zl.With(zap.Error(err))}
}

func (l *zapLogger) convert(fields []Field) []zap.Field {
	out := make([]zap.Field, len(fields))
	for i, f := range fields {
		out[i] = toZap(f)
	}
	return out
}

func toZap(f Field) zap.Field {
	switch v := f.Value.(type) {
	case string:
		return zap.String(f.Key, v)
	case int:
		return zap.Int(f.Key, v)
	case int64:
		return zap.Int64(f.Key, v)
	case float64:
		return zap.Float64(f.Key, v)
	case bool:
		return zap.Bool(f.Key, v)
	case time.Duration:
		return zap.Duration(f.Key, v)
	case time.Time:
		return zap.Time(f.Key, v)
	case error:
		// Keep the reserved key to match the Error() helper.
		return zap.NamedError(f.Key, v)
	default:
		return zap.Any(f.Key, v)
	}
}
