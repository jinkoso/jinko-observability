# jinko-observability

Shared observability primitives for Jinko Go services — logging, tracing, baggage, HTTP middleware, and async-job trace continuity.

Consumed by `jinko-connector` and `jinko-mcp-bff` (and any future Go service). One contract, one implementation, versioned via semver.

## Status

| Package | Purpose | Status |
|---|---|---|
| `pkg/conventions` | Single source of truth for header names, baggage keys, span attributes, and service names. | v0.2.0 |
| `pkg/logger` | Zap-backed structured logger with OTel trace-ID correlation (Datadog-compatible) and identity-baggage helpers. | v0.2.0 |
| `pkg/telemetry` | OTel SDK init: TracerProvider, OTLP/gRPC exporter, W3C trace+baggage propagator, baggagecopy SpanProcessor. Plus identity baggage helpers. | v0.2.0 |
| `pkg/httpx` | Gin server middleware (otelgin), traced HTTP client for inter-service calls, and a baggage-stripping client for third-party calls. | v0.2.0 |
| `pkg/jobs` | TraceCarrier + Inject/Extract for propagating W3C trace context across async job queue boundaries (River, etc.). | v0.2.0 |
| `pkg/health` | Standard `/health`, `/health/live`, `/health/ready` handlers with pluggable readiness checks. | Planned |

**Stability**: v0.2.x is marked Experimental. APIs may change without notice during the v0.2.x line. Pin to a tagged release.

## Quick start

A minimal Gin service wired for full Jinko-standard observability:

```go
package main

import (
    "context"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jinkoso/jinko-observability/pkg/httpx"
    "github.com/jinkoso/jinko-observability/pkg/logger"
    "github.com/jinkoso/jinko-observability/pkg/telemetry"
)

func main() {
    ctx := context.Background()

    shutdown, err := telemetry.Init(ctx, telemetry.Config{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        OTLPEndpoint:   "datadog-agent:4317",
        Insecure:       true,
        Enabled:        true,
    })
    if err != nil { panic(err) }
    defer shutdown(context.Background())

    log := logger.MustNew(logger.DefaultConfig())

    r := gin.New()
    r.Use(httpx.Middleware("my-service"))
    r.GET("/hello", func(c *gin.Context) {
        ctx := c.Request.Context()
        log.With(logger.IdentityFields(ctx)...).Info(ctx, "handling /hello")
        c.JSON(http.StatusOK, gin.H{"ok": true})
    })

    _ = r.Run(":8080")
}
```

Inter-service calls:
```go
client := httpx.NewTracedClient(httpx.ClientConfig{Timeout: 30 * time.Second})
// W3C traceparent + baggage propagated automatically.
```

Third-party calls (Stripe, OpenAI, Sabre, ...):
```go
stripeHTTP := httpx.NewExternalClient(httpx.ClientConfig{Timeout: 30 * time.Second}, "stripe")
// traceparent propagated; baggage stripped to keep user.id/tenant.id off the vendor's servers.
```

Identity baggage at the trust boundary:
```go
ctx = telemetry.SetIdentity(ctx, telemetry.Identity{
    UserID:    authResult.UserID,
    TenantID:  authResult.TenantID,
    SessionID: authResult.SessionID,
    RequestID: requestID,
})
// Every span downstream is auto-tagged with user.id, tenant.id, etc.
// via the baggagecopy SpanProcessor.
```

Async jobs (River, etc.):
```go
// producer:
args := FulfillItemArgs{CartID: id, Trace: jobs.Inject(ctx)}
_, _ = riverClient.Insert(ctx, args, ...)

// consumer:
ctx, parent := jobs.Extract(ctx, args.Trace)
tracer := otel.Tracer("river-worker")
opts := []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindConsumer)}
if parent.IsValid() {
    opts = append(opts, trace.WithLinks(trace.Link{SpanContext: parent}))
}
ctx, span := tracer.Start(ctx, "fulfill_item", opts...)
defer span.End()
```

## Sampling

The library configures **100% SDK sampling** (`parentbased_always_on`). Long-term retention is controlled by Datadog Retention Filters at the backend, not by SDK head sampling. Spans dropped by an SDK sampler can never be saved by retention filters — they never reach Datadog.

If ingestion cost requires reduction, add tail-based sampling at the OTel Collector / DDOT layer instead. The library does not expose a head-sampling rate to discourage misuse.

## Trace ↔ Log correlation

The `pkg/logger` package emits **both** OTel-native and Datadog-legacy field names on every log line that runs with an active span:

- `trace_id` (32-char lowercase hex) and `span_id` (16-char lowercase hex) — OTel-native
- `dd.trace_id` and `dd.span_id` (decimal) — Datadog legacy format

Datadog auto-links logs to APM traces when either pair is present.

## Versioning

Semver. Breaking changes bump the major version. Both services pin to a tagged release.

## License

Proprietary — Jinko internal.
