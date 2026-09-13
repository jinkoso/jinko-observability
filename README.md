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

**v0.5.0 (unreleased)**: `pkg/conventions` exports the canonical Datadog `env` vocabulary (`dev` / `sandbox` / `preprod` / `prod`) plus `IsCanonicalEnvironment`, and `telemetry.Init` now rejects any other `Environment` value when `Enabled` is true (a disabled config is not validated) (JIN-1570). Services still passing `production`, `prod-us`, or `staging` with telemetry enabled fail at startup and must switch to a canonical value. The check covers the merged resource, so `ExtraResourceAttributes` cannot override `deployment.environment` or `env` with a non-canonical value either.

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
        Environment:    "prod", // dev | sandbox | preprod | prod
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

## Environment vocabulary

The Datadog `env` tag has exactly four legal values across the estate:

| Value | Deployment |
|---|---|
| `dev` | Development environment |
| `sandbox` | Customer-facing sandbox |
| `preprod` | Pre-production |
| `prod` | Production (all regions) |

Region, cluster, and cell go in the host/cluster tags — never in `env`. A value like `prod-us` splits one logical environment across two APM buckets and breaks every cross-service query, dashboard, and monitor that groups by env.

When `Enabled` is true, `telemetry.Init` validates `Config.Environment` against this set and returns an error for anything else, so a misconfigured service fails at startup instead of shipping traces nobody queries (a disabled config is a no-op and is not validated — it exports no spans, so it has no `env` tag to get wrong):

```
telemetry: Environment "prod-us" is not canonical; use one of dev, sandbox, preprod, prod (JIN-1570)
```

`ExtraResourceAttributes` is appended after the defaults, so it could otherwise
overwrite the env tag after that check had passed. The merged resource is
re-checked for the same reason:

```
telemetry: resource deployment.environment "production" is not canonical; use one of dev, sandbox, preprod, prod, and set it through Config.Environment rather than ExtraResourceAttributes (JIN-1570)
```

The values are exported as constants for use in service config code:

```go
conventions.EnvDev      // "dev"
conventions.EnvSandbox  // "sandbox"
conventions.EnvPreprod  // "preprod"
conventions.EnvProd     // "prod"

conventions.Environments                    // []string{"dev", "sandbox", "preprod", "prod"}
conventions.IsCanonicalEnvironment("staging") // false
```

Matching is exact — no case folding, no whitespace trimming. `Prod` and `prod ` are rejected.

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
