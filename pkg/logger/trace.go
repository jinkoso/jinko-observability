package logger

import (
	"context"
	"encoding/binary"
	"strconv"

	"go.opentelemetry.io/otel/trace"
)

// Datadog-reserved attribute names for log↔APM trace correlation.
// See: https://docs.datadoghq.com/tracing/connect_logs_and_traces/opentelemetry/
const (
	ddTraceIDKey = "dd.trace_id"
	ddSpanIDKey  = "dd.span_id"

	otelTraceIDKey = "trace_id"
	otelSpanIDKey  = "span_id"
)

// TraceContext holds both the OpenTelemetry hex encoding and the
// Datadog-compatible decimal encoding for a span.
type TraceContext struct {
	TraceID      string // OTel 32-char hex
	SpanID       string // OTel 16-char hex
	DDTraceIDVal string // decimal, lower 64 bits of TraceID
	DDSpanIDVal  string // decimal, full 64-bit SpanID
}

// ExtractTraceContext pulls trace/span IDs from ctx if a valid OTel span
// is present. Returns the zero value when ctx carries no span.
func ExtractTraceContext(ctx context.Context) TraceContext {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return TraceContext{}
	}

	tid := sc.TraceID()
	sid := sc.SpanID()

	return TraceContext{
		TraceID:      tid.String(),
		SpanID:       sid.String(),
		DDTraceIDVal: strconv.FormatUint(binary.BigEndian.Uint64(tid[8:]), 10),
		DDSpanIDVal:  strconv.FormatUint(binary.BigEndian.Uint64(sid[:]), 10),
	}
}

// TraceFieldsFromContext returns logging fields carrying trace and span
// IDs in both OTel and Datadog formats. Empty slice when ctx has no span.
func TraceFieldsFromContext(ctx context.Context) []Field {
	tc := ExtractTraceContext(ctx)
	if tc.TraceID == "" {
		return nil
	}
	return []Field{
		String(ddTraceIDKey, tc.DDTraceIDVal),
		String(ddSpanIDKey, tc.DDSpanIDVal),
		String(otelTraceIDKey, tc.TraceID),
		String(otelSpanIDKey, tc.SpanID),
	}
}

// withTrace prepends trace fields (if any) to the given field slice.
// Internal to the package.
func withTrace(ctx context.Context, fields []Field) []Field {
	tf := TraceFieldsFromContext(ctx)
	if len(tf) == 0 {
		return fields
	}
	out := make([]Field, 0, len(tf)+len(fields))
	out = append(out, tf...)
	out = append(out, fields...)
	return out
}
