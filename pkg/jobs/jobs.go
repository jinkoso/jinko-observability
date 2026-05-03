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
	"go.opentelemetry.io/otel/baggage"
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

// Extract restores baggage from the carrier onto ctx and returns the
// producer's SpanContext separately for use with trace.WithLinks. The
// returned ctx does NOT carry the producer span — callers must start
// their worker span from a clean parentage so the worker becomes a new
// trace root, linked (not parented) to the producer.
//
// Worker pattern:
//
//	ctx, parent := jobs.Extract(ctx, args.Trace)
//	tracer := otel.Tracer("river-worker")
//	opts := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindConsumer)}
//	if parent.IsValid() {
//	    opts = append(opts, trace.WithLinks(trace.Link{SpanContext: parent}))
//	}
//	ctx, span := tracer.Start(ctx, "river.fulfill_item", opts...)
//	defer span.End()
//
// Why detach: if the producer span stayed on ctx, tracer.Start would
// make the worker span a CHILD of the producer rather than linking to
// it. The resulting trace would span the entire queue lag (potentially
// hours), distorting latency analysis.
//
// Baggage IS carried over: identity (user.id, tenant.id, etc.) flows
// to the worker so logs and child spans remain attributable.
//
// When c is empty (a legacy job enqueued before this plumbing
// existed), Extract returns ctx unchanged and an invalid SpanContext.
//
// When c carries baggage but no traceparent (or an unparseable one),
// Extract still merges the baggage onto ctx and returns an invalid
// SpanContext — the caller's pre-existing span on ctx is NEVER used
// as the link target.
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

	// Extract from a clean Background ctx so the producer span and
	// baggage are isolated from anything already on the caller's ctx.
	clean := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

	// Producer span comes ONLY from the carrier — never from caller's ctx.
	parent := trace.SpanContextFromContext(clean)

	// Merge baggage onto caller's ctx. Existing entries are preserved;
	// carrier entries with the same key overwrite.
	out := ctx
	if b := baggage.FromContext(clean); b.Len() > 0 {
		merged := baggage.FromContext(out)
		for _, m := range b.Members() {
			if next, err := merged.SetMember(m); err == nil {
				merged = next
			}
		}
		out = baggage.ContextWithBaggage(out, merged)
	}

	return out, parent
}
