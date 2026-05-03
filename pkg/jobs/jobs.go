// Package jobs provides helpers for propagating OpenTelemetry trace
// context across asynchronous job boundaries — typically a durable job
// queue like River where the producer and consumer run in different
// processes minutes or hours apart.
//
// The standard pattern: an HTTP handler (producer) builds a TraceCarrier
// from its context with Inject and embeds it in the job payload. The
// worker (consumer) calls Extract to restore the trace and baggage on
// its own context, then starts a fresh span linked to the original.
//
// Workers should NOT continue the original span — that produces a single
// long-running trace whose duration is the queue lag. Use
// trace.WithLinks(parentSpanContext) on a new span instead, which makes
// the relationship visible in Datadog APM as a "linked traces" badge
// without distorting latency.
//
// Stability: Experimental.
package jobs

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceCarrier is the serialized W3C trace + baggage context attached
// to a job payload. JSON-friendly fields; empty fields are omitted.
//
// Embed TraceCarrier as a named field in your job args struct:
//
//	type FulfillItemArgs struct {
//	    CartID int64           `json:"cart_id"`
//	    Trace  jobs.TraceCarrier `json:"trace,omitempty"`
//	}
type TraceCarrier struct {
	Traceparent string `json:"traceparent,omitempty"`
	Tracestate  string `json:"tracestate,omitempty"`
	Baggage     string `json:"baggage,omitempty"`
}

// IsZero reports whether the carrier is empty — i.e. the producing
// context had no active trace or baggage. Useful for backward-compat
// checks when consuming jobs that pre-date the trace plumbing.
func (c TraceCarrier) IsZero() bool {
	return c.Traceparent == "" && c.Tracestate == "" && c.Baggage == ""
}

// Inject builds a TraceCarrier from the active trace context and
// baggage on ctx. Producers call this immediately before enqueueing a
// job:
//
//	args := FulfillItemArgs{CartID: id, Trace: jobs.Inject(ctx)}
//	_, err := river.Insert(ctx, args, ...)
//
// Inject uses the global TextMapPropagator (set by pkg/telemetry.Init)
// so propagation behaves identically to inter-service HTTP calls.
func Inject(ctx context.Context) TraceCarrier {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return TraceCarrier{
		Traceparent: carrier["traceparent"],
		Tracestate:  carrier["tracestate"],
		Baggage:     carrier["baggage"],
	}
}

// Extract restores the trace context and baggage carried by c onto a
// fresh context derived from ctx. The returned SpanContext is the
// original producer span — pass it to trace.WithLinks when starting a
// worker span:
//
//	ctx, parent := jobs.Extract(ctx, args.Trace)
//	tracer := otel.Tracer("river-worker")
//	var opts []trace.SpanStartOption
//	opts = append(opts, trace.WithSpanKind(trace.SpanKindConsumer))
//	if parent.IsValid() {
//	    opts = append(opts, trace.WithLinks(trace.Link{SpanContext: parent}))
//	}
//	ctx, span := tracer.Start(ctx, "river.fulfill_item", opts...)
//	defer span.End()
//
// When c is empty (e.g. a job enqueued before this plumbing existed),
// Extract returns ctx unchanged and an invalid SpanContext.
func Extract(ctx context.Context, c TraceCarrier) (context.Context, trace.SpanContext) {
	if c.IsZero() {
		return ctx, trace.SpanContext{}
	}
	carrier := propagation.MapCarrier{}
	if c.Traceparent != "" {
		carrier["traceparent"] = c.Traceparent
	}
	if c.Tracestate != "" {
		carrier["tracestate"] = c.Tracestate
	}
	if c.Baggage != "" {
		carrier["baggage"] = c.Baggage
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	return ctx, trace.SpanContextFromContext(ctx)
}
