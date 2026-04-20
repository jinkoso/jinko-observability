# jinko-observability

Shared observability primitives for Jinko Go services — logging, metrics, tracing, and health.

Consumed by `jinko-connector` and `jinko-mcp-bff`. One contract, one implementation, versioned via semver.

## Status

| Package | Purpose | Status |
|---|---|---|
| `pkg/logger` | Zap-backed structured logger with OTel trace-ID correlation (Datadog-compatible). | Phase 1 |
| `pkg/telemetry` | OTLP/gRPC tracer + meter init with delta-temporality, shared resource builder. | Planned |
| `pkg/httpx` | Gin HTTP middleware recording OTel-convention metrics + per-request tracing. | Planned |
| `pkg/health` | Standard `/health`, `/health/live`, `/health/ready` handlers with pluggable readiness checks. | Planned |

## Usage (phase 1 — logger only)

```go
import "github.com/jinkoso/jinko-observability/pkg/logger"

log, err := logger.New(logger.Config{
    Level:  "info",
    Format: "json",
    Output: "stdout",
})
if err != nil {
    panic(err)
}

ctx := context.Background()
log.Info(ctx, "service starting",
    logger.String("version", "1.0.0"),
    logger.Int("port", 8080),
)
```

Trace correlation: when the incoming `ctx` carries an OpenTelemetry span, the logger automatically emits `dd.trace_id`, `dd.span_id`, `trace_id`, and `span_id` fields — Datadog links logs to APM traces with no extra wiring.

## Versioning

Semver. Breaking changes bump the major version. Both services pin to a tagged release.

## License

Proprietary — Jinko internal.
