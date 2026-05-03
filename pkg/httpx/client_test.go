package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jinkoso/jinko-observability/pkg/httpx"
	"github.com/jinkoso/jinko-observability/pkg/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// captureTransport records the headers of every outbound request so
// tests can assert on what would actually go on the wire.
type captureTransport struct {
	lastReq *http.Request
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.lastReq = req.Clone(req.Context())
	return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
}

// setupTracing installs a real (non-no-op) TracerProvider AND the W3C
// composite propagator on the global OTel APIs for the duration of the
// test. Without a real provider, otelhttp injects nothing because the
// default no-op tracer produces invalid span contexts.
func setupTracing(t *testing.T) {
	t.Helper()

	prevTP := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider() // no exporter — we only need valid IDs
	otel.SetTracerProvider(tp)

	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
		_ = tp.Shutdown(context.Background())
	})
}

func TestNewTracedClient_PropagatesBaggage(t *testing.T) {
	setupTracing(t)

	cap := &captureTransport{}
	client := httpx.NewTracedClient(httpx.ClientConfig{BaseTransport: cap})

	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{
		UserID:   "u_123",
		TenantID: "acme",
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/whatever", nil)
	_, err := client.Do(req)
	require.NoError(t, err)

	require.NotNil(t, cap.lastReq)
	bag := cap.lastReq.Header.Get("baggage")
	assert.Contains(t, bag, "user.id=u_123", "internal client must propagate user.id baggage")
	assert.Contains(t, bag, "tenant.id=acme", "internal client must propagate tenant.id baggage")
}

func TestNewExternalClient_StripsBaggage(t *testing.T) {
	setupTracing(t)

	cap := &captureTransport{}
	client := httpx.NewExternalClient(httpx.ClientConfig{BaseTransport: cap}, "stripe")

	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{
		UserID:   "u_should_not_leak",
		TenantID: "acme_should_not_leak",
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://api.stripe.com/v1/charges", nil)
	_, err := client.Do(req)
	require.NoError(t, err)

	require.NotNil(t, cap.lastReq)
	assert.Empty(t, cap.lastReq.Header.Get("baggage"),
		"external client MUST NOT send the baggage header — identity leak to third party")
}

func TestNewExternalClient_StillPropagatesTraceContext(t *testing.T) {
	// The whole point of NewExternalClient (vs an unwrapped http.Client)
	// is keeping the trace continuity intact. Strip baggage, KEEP traceparent.
	setupTracing(t)

	cap := &captureTransport{}
	client := httpx.NewExternalClient(httpx.ClientConfig{BaseTransport: cap}, "stripe")

	// otelhttp injects traceparent only when there's an active span. Use a
	// real ctx with a dummy parent context so we hit the inject path.
	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{UserID: "u_x"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://api.stripe.com/health", nil)
	_, err := client.Do(req)
	require.NoError(t, err)

	require.NotNil(t, cap.lastReq)
	// otelhttp will have started a client span and injected traceparent.
	assert.NotEmpty(t, cap.lastReq.Header.Get("traceparent"),
		"external client must propagate traceparent for vendor-call trace continuity")
}

func TestNewTracedClient_AppliesDefaultTimeout(t *testing.T) {
	c := httpx.NewTracedClient(httpx.ClientConfig{})
	// Package-private default is 30s. Keep in sync with httpx/httpx.go.
	assert.Equal(t, 30*time.Second, c.Timeout, "zero timeout must fall back to default")
}

func TestExternalClient_RealServer_StripsBaggageE2E(t *testing.T) {
	// Hit a real httptest server end-to-end so we exercise the full
	// http.Transport→tls→server roundtrip path. This is the most
	// important regression for the "no PII to vendors" guarantee.
	setupTracing(t)

	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := httpx.NewExternalClient(httpx.ClientConfig{}, "test-vendor")
	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{UserID: "must_not_leak"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	_, err := client.Do(req)
	require.NoError(t, err)

	assert.Empty(t, got.Get("baggage"))
	assert.NotEmpty(t, got.Get("traceparent"))
}
