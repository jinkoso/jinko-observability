package httpx

import (
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// ClientConfig configures the HTTP client constructors.
type ClientConfig struct {
	// Timeout is the per-request timeout. Zero falls back to a 30s default.
	Timeout time.Duration

	// MaxIdleConns caps the total idle connection pool. Zero falls back
	// to 100.
	MaxIdleConns int

	// MaxIdleConnsPerHost caps idle connections per remote host. Zero
	// falls back to 10.
	MaxIdleConnsPerHost int

	// IdleConnTimeout is how long an idle connection lives before being
	// closed. Zero falls back to 90s.
	IdleConnTimeout time.Duration

	// BaseTransport, if set, is the http.RoundTripper wrapped by the
	// otelhttp transport. Useful for tests (e.g. in-memory transport)
	// or for layering a debug-capture transport. Nil uses a configured
	// http.Transport built from the other fields.
	BaseTransport http.RoundTripper
}

// NewTracedClient returns an *http.Client suitable for inter-service
// calls inside the Jinko trust boundary (BFF→connector, BFF→worker,
// etc.). It injects W3C traceparent AND baggage into outbound headers
// via the global propagator.
//
// Use NewExternalClient instead when calling third parties (Stripe,
// Sabre, TravelFusion, OpenAI, Resend, ...) so identity baggage does
// not leak outside the trust boundary.
func NewTracedClient(cfg ClientConfig) *http.Client {
	base := baseTransport(cfg)
	return &http.Client{
		Timeout:   timeoutOrDefault(cfg.Timeout),
		Transport: otelhttp.NewTransport(base),
	}
}

// NewExternalClient returns an *http.Client for calling third parties.
// It injects the trace context (so vendor calls show up nested in the
// APM trace) but strips the W3C baggage header before each request, so
// internal identity (user.id, tenant.id, etc.) is not leaked to the
// vendor's logs and SIEM.
//
// destination is a low-cardinality label used as the otelhttp peer
// service name (e.g. "stripe", "travelfusion"). Pass the same value
// for every call to a given vendor.
func NewExternalClient(cfg ClientConfig, destination string) *http.Client {
	base := baseTransport(cfg)
	// Wrap order matters: otelhttp injects headers (traceparent + baggage)
	// from the active context, then the baggageStripper deletes "baggage"
	// from the request before it reaches the network. Putting the stripper
	// OUTSIDE otelhttp would let otelhttp re-add baggage after the strip.
	stripped := &baggageStripper{base: base}
	return &http.Client{
		Timeout: timeoutOrDefault(cfg.Timeout),
		Transport: otelhttp.NewTransport(stripped,
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				if destination != "" {
					return destination + " " + r.Method
				}
				return r.Method
			}),
		),
	}
}

func baseTransport(cfg ClientConfig) http.RoundTripper {
	if cfg.BaseTransport != nil {
		return cfg.BaseTransport
	}
	idleTimeout := cfg.IdleConnTimeout
	if idleTimeout == 0 {
		idleTimeout = defaultIdleConnTimeout
	}
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        intOrDefault(cfg.MaxIdleConns, defaultMaxIdleConns),
		MaxIdleConnsPerHost: intOrDefault(cfg.MaxIdleConnsPerHost, defaultMaxIdleConnsPerHost),
		IdleConnTimeout:     idleTimeout,
		ForceAttemptHTTP2:   true,
	}
}

func timeoutOrDefault(t time.Duration) time.Duration {
	if t <= 0 {
		return defaultTimeout
	}
	return t
}

func intOrDefault(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
