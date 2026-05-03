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

	// And the propagator put the span back in the restored ctx so further
	// otel.SpanFromContext works downstream.
	fromCtx := trace.SpanContextFromContext(restored)
	assert.Equal(t, originalSpanCtx.TraceID(), fromCtx.TraceID())
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
