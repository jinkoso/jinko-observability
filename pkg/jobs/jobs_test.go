package jobs_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jinkoso/jinko-observability/pkg/conventions"
	"github.com/jinkoso/jinko-observability/pkg/jobs"
	"github.com/jinkoso/jinko-observability/pkg/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// installPropagators sets up real OTel globals for the test. Without a
// real TracerProvider, otelhttp/Inject produce empty traceparents.
func installPropagators(t *testing.T) trace.TracerProvider {
	t.Helper()
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
		_ = tp.Shutdown(context.Background())
	})
	return tp
}

func TestInject_EmptyContext_ProducesZeroCarrier(t *testing.T) {
	installPropagators(t)
	c := jobs.Inject(context.Background())
	assert.True(t, c.IsZero(), "no span and no baggage should produce a zero carrier")
}

func TestInject_BaggageOnlyRoundTrip(t *testing.T) {
	installPropagators(t)

	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{
		UserID:   "u_async",
		TenantID: "acme",
	})

	carrier := jobs.Inject(ctx)
	assert.NotEmpty(t, carrier.Baggage, "baggage must be carried even without an active span")
	assert.Empty(t, carrier.Traceparent, "no active span -> no traceparent")

	// Round-trip back into a fresh ctx and confirm identity baggage survives.
	restored, parent := jobs.Extract(context.Background(), carrier)
	assert.False(t, parent.IsValid(), "no traceparent -> invalid SpanContext")
	got := telemetry.ReadIdentity(restored)
	assert.Equal(t, "u_async", got.UserID)
	assert.Equal(t, "acme", got.TenantID)
}

func TestInject_TraceContextRoundTrip(t *testing.T) {
	tp := installPropagators(t)

	tracer := tp.Tracer("jobs-test")
	ctx, span := tracer.Start(context.Background(), "producer")
	originalSpanCtx := span.SpanContext()
	span.End()

	carrier := jobs.Inject(ctx)
	require.NotEmpty(t, carrier.Traceparent, "active span must produce a traceparent")

	restored, parent := jobs.Extract(context.Background(), carrier)
	require.True(t, parent.IsValid(), "extracted SpanContext must be valid")
	assert.Equal(t, originalSpanCtx.TraceID(), parent.TraceID(),
		"trace_id must survive Inject/Extract")
	assert.Equal(t, originalSpanCtx.SpanID(), parent.SpanID(),
		"the producer's span_id is the parent for the linked worker span")

	// CRITICAL: the returned ctx MUST NOT carry the producer span. Otherwise
	// tracer.Start would make the worker span a CHILD of the producer span,
	// merging queue lag into the trace duration. Worker code must be free
	// to start a fresh root span linked to the producer.
	assert.False(t, trace.SpanContextFromContext(restored).IsValid(),
		"Extract must NOT leave the producer span on the returned ctx")
}

// TestExtract_WorkerSpan_HasIndependentTrace is the load-bearing
// regression test for the link-vs-child contract. It documents the
// canonical worker pattern and asserts that the resulting worker span
// is in its OWN trace, NOT a child of the producer trace.
//
// If this test ever flips, the entire "linked traces" UX in Datadog
// APM breaks: workers running hours after their producer would appear
// as one trace whose duration is the queue lag.
func TestExtract_WorkerSpan_HasIndependentTrace(t *testing.T) {
	tp := installPropagators(t)
	tracer := tp.Tracer("jobs-test")

	// 1. Producer side: start a span, inject into a carrier.
	prodCtx, prodSpan := tracer.Start(context.Background(), "http.handler")
	prodSpanCtx := prodSpan.SpanContext()
	carrier := jobs.Inject(prodCtx)
	prodSpan.End()

	// 2. Worker side: extract, then run the documented WithLinks pattern.
	workerInputCtx := context.Background() // worker starts with a clean ctx
	workerCtx, parent := jobs.Extract(workerInputCtx, carrier)

	require.True(t, parent.IsValid(), "carrier must yield a valid parent SpanContext")

	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
	}
	if parent.IsValid() {
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: parent}))
	}
	_, workerSpan := tracer.Start(workerCtx, "river.fulfill_item", opts...)
	defer workerSpan.End()

	workerSpanCtx := workerSpan.SpanContext()

	assert.NotEqual(t, prodSpanCtx.TraceID(), workerSpanCtx.TraceID(),
		"worker span must be in a DIFFERENT trace from the producer "+
			"(otherwise queue lag pollutes trace duration)")
	assert.True(t, workerSpanCtx.IsValid(), "worker span context must be valid")
}

// TestExtract_BaggageOnlyCarrier_DoesNotPickUpCallerSpan asserts that
// when the carrier has baggage but no traceparent (e.g. an unparseable
// or missing one), Extract returns an invalid SpanContext rather than
// silently latching onto whatever span happens to be on the caller's
// ctx.
//
// Without this guarantee, a worker framework that wraps Work() with
// its own span would have that span erroneously become the link target.
func TestExtract_BaggageOnlyCarrier_DoesNotPickUpCallerSpan(t *testing.T) {
	tp := installPropagators(t)
	tracer := tp.Tracer("jobs-test")

	// Caller's ctx has its own (unrelated) span — say, a worker-framework span.
	callerCtx, framingSpan := tracer.Start(context.Background(), "river.framing")
	defer framingSpan.End()

	// Carrier has baggage but no traceparent.
	carrier := jobs.TraceCarrier{
		Baggage: conventions.BaggageUserID + "=u_x",
	}

	_, parent := jobs.Extract(callerCtx, carrier)
	assert.False(t, parent.IsValid(),
		"carrier without traceparent must yield invalid SpanContext, "+
			"NOT the caller's pre-existing span")
}

func TestExtract_ZeroCarrier_LeavesContextUntouched(t *testing.T) {
	installPropagators(t)

	// Prepare a ctx with an unrelated baggage entry to verify it's preserved.
	mem, err := baggage.NewMemberRaw("custom.flag", "yes")
	require.NoError(t, err)
	b, err := baggage.New(mem)
	require.NoError(t, err)
	ctxIn := baggage.ContextWithBaggage(context.Background(), b)

	ctxOut, parent := jobs.Extract(ctxIn, jobs.TraceCarrier{})
	assert.False(t, parent.IsValid())
	assert.Equal(t, "yes", baggage.FromContext(ctxOut).Member("custom.flag").Value(),
		"empty carrier must not clobber pre-existing baggage on ctx")
}

func TestExtract_PreservesCallerBaggage(t *testing.T) {
	// When carrier has baggage AND caller already has different baggage,
	// both must survive (caller's existing entries + carrier's new ones).
	installPropagators(t)

	preMember, err := baggage.NewMemberRaw("custom.flag", "yes")
	require.NoError(t, err)
	preBag, err := baggage.New(preMember)
	require.NoError(t, err)
	callerCtx := baggage.ContextWithBaggage(context.Background(), preBag)

	// Producer-side baggage (only user.id):
	prodCtx := telemetry.SetIdentity(context.Background(), telemetry.Identity{
		UserID: "u_from_carrier",
	})
	carrier := jobs.Inject(prodCtx)

	merged, _ := jobs.Extract(callerCtx, carrier)
	mergedBag := baggage.FromContext(merged)

	assert.Equal(t, "yes", mergedBag.Member("custom.flag").Value(),
		"caller's pre-existing baggage must survive merge")
	assert.Equal(t, "u_from_carrier", mergedBag.Member(conventions.BaggageUserID).Value(),
		"carrier's baggage must be added on top")
}

func TestTraceCarrier_JSONShape(t *testing.T) {
	// Nail the on-wire shape — the field names matter for cross-service
	// compatibility and for old workers reading new payloads.
	c := jobs.TraceCarrier{
		Traceparent: "00-trace-span-01",
		Baggage:     conventions.BaggageUserID + "=u_x",
	}
	out, err := json.Marshal(c)
	require.NoError(t, err)
	assert.JSONEq(t, `{"traceparent":"00-trace-span-01","baggage":"user.id=u_x"}`, string(out))

	// Empty carrier marshals to {} thanks to omitempty — keeps payloads tidy.
	out, err = json.Marshal(jobs.TraceCarrier{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(out))
}
