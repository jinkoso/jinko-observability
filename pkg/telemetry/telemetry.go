// Package telemetry initializes OpenTelemetry for Jinko Go services.
//
// A single Init call configures the global TracerProvider, the global
// composite TextMapPropagator (W3C trace context + W3C baggage), and a
// baggagecopy SpanProcessor that promotes identity baggage onto every
// span attribute. The returned shutdown function flushes the batch
// exporter and must be deferred from main.
//
// Sampling: 100% at the SDK layer (parentbased_always_on). Long-term
// retention is controlled by Datadog retention filters, not by the
// SDK. Head sampling at this layer is intentionally not exposed —
// dropped spans never reach Datadog and therefore cannot be saved by
// retention rules. If ingestion cost requires reduction, add tail
// sampling at the Collector layer instead.
//
// Stability: Experimental.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jinkoso/jinko-observability/pkg/conventions"

	"go.opentelemetry.io/contrib/processors/baggagecopy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config controls Init. ServiceName, Environment, and OTLPEndpoint are
// required when Enabled is true; ServiceVersion is optional but
// recommended for Datadog Unified Service Tagging.
type Config struct {
	// ServiceName populates OTel service.name and Datadog "service" tag.
	ServiceName string

	// ServiceVersion populates OTel service.version and Datadog "version".
	ServiceVersion string

	// Environment populates OTel deployment.environment and Datadog "env".
	// Must be one of the canonical values in conventions.Environments —
	// "dev", "sandbox", "preprod", "prod" (JIN-1570). Region and cluster
	// belong in host/cluster tags, not here.
	Environment string

	// OTLPEndpoint is the gRPC endpoint of the Datadog Agent OTLP receiver
	// (or the DDOT Collector). Typically "datadog-agent:4317" in cluster
	// or "localhost:4317" in local dev. No scheme.
	OTLPEndpoint string

	// Insecure disables TLS for the OTLP exporter. True for local /
	// in-cluster DD Agent (the typical case).
	Insecure bool

	// ExporterTimeout caps the OTLP gRPC export call duration. Zero
	// uses the SDK default (10s).
	ExporterTimeout time.Duration

	// Enabled is the master switch. When false, Init returns a no-op
	// shutdown and leaves the global providers untouched. Useful for
	// tests and `make dev` runs without an Agent.
	Enabled bool

	// BaggageKeyPrefixes overrides which baggage keys the SpanProcessor
	// promotes to span attributes. Defaults to conventions.BaggageKeyPrefixes
	// when nil. Pass an empty slice to disable promotion entirely.
	BaggageKeyPrefixes []string

	// Resource attributes appended on top of the defaults built from
	// ServiceName/ServiceVersion/Environment. Use for service.namespace,
	// host.name, etc.
	//
	// These are appended LAST, so they override the defaults — with one
	// exception: deployment.environment and env stay under the canonical
	// vocabulary check (JIN-1570). buildResource re-checks the merged
	// resource and fails, rather than let an override slip a value past
	// validate. Setting the environment belongs in Environment above.
	ExtraResourceAttributes []sdkresource.Option
}

// Shutdown flushes any buffered spans and releases resources. Safe to
// call multiple times; safe when Init was a no-op (returns nil).
type Shutdown func(context.Context) error

// Init configures the global OTel SDK. It returns a Shutdown function
// that the caller must defer.
//
// The global composite TextMapPropagator (W3C trace context + W3C
// baggage) is registered UNCONDITIONALLY — even when cfg.Enabled is
// false. A disabled service still participates in cross-service tracing
// context: it must propagate the incoming traceparent and baggage to
// its downstream calls so traces aren't broken at the boundary. Only the
// exporter, TracerProvider, and sampler are gated on Enabled.
//
// When cfg.Enabled is false, Init returns a no-op Shutdown and does not
// install a TracerProvider — the program runs without exporting spans.
func Init(ctx context.Context, cfg Config) (Shutdown, error) {
	// Register the propagator regardless of Enabled so disabled services
	// still inject/extract traceparent + baggage on their HTTP calls.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // traceparent + tracestate
		propagation.Baggage{},      // W3C baggage
	))

	if !cfg.Enabled {
		return noopShutdown, nil
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	exp, err := buildTraceExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build trace exporter: %w", err)
	}

	prefixes := cfg.BaggageKeyPrefixes
	if prefixes == nil {
		prefixes = conventions.BaggageKeyPrefixes
	}
	baggageFilter := newBaggageFilter(prefixes)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		// 100% SDK sampling. Retention is controlled by Datadog filters.
		// ParentBased ensures child services honor an upstream sampling
		// decision so traces don't get half-dropped at boundaries.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithSpanProcessor(baggagecopy.NewSpanProcessor(baggageFilter)),
		sdktrace.WithBatcher(exp),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func (c Config) validate() error {
	if c.ServiceName == "" {
		return errors.New("telemetry: ServiceName is required")
	}
	if c.Environment == "" {
		// Required for Datadog Unified Service Tagging. Failing fast here
		// prevents prod from silently shipping traces without the `env`
		// tag, which would mix prod/preprod/dev together in APM.
		return fmt.Errorf("telemetry: Environment is required (one of %s)",
			strings.Join(conventions.Environments, ", "))
	}
	if !conventions.IsCanonicalEnvironment(c.Environment) {
		// A non-canonical value is worse than a missing one: the service ships
		// traces under an env nobody queries ("production", "prod-us"), so its
		// spans are absent from every estate-wide dashboard and monitor while
		// looking perfectly healthy locally.
		return fmt.Errorf("telemetry: Environment %q is not canonical; use one of %s (JIN-1570)",
			c.Environment, strings.Join(conventions.Environments, ", "))
	}
	if c.OTLPEndpoint == "" {
		return errors.New("telemetry: OTLPEndpoint is required")
	}
	return nil
}

func buildResource(ctx context.Context, cfg Config) (*sdkresource.Resource, error) {
	attrs := []sdkresource.Option{
		sdkresource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, sdkresource.WithAttributes(
			semconv.ServiceVersion(cfg.ServiceVersion),
		))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, sdkresource.WithAttributes(
			semconv.DeploymentEnvironment(cfg.Environment),
		))
	}
	attrs = append(attrs, cfg.ExtraResourceAttributes...)

	res, err := sdkresource.New(ctx, attrs...)
	if err != nil {
		return nil, err
	}
	if err := validateResourceEnvironment(res); err != nil {
		return nil, err
	}
	return res, nil
}

// environmentResourceKeys are the resource attributes that become the Datadog
// `env` tag: the OTel semantic-convention key, and the literal key Datadog
// Unified Service Tagging reads when a caller sets it directly.
var environmentResourceKeys = map[attribute.Key]bool{
	semconv.DeploymentEnvironmentKey: true,
	attribute.Key("env"):             true,
}

// validateResourceEnvironment re-applies the canonical-vocabulary check to the
// MERGED resource. Config.validate only sees cfg.Environment, but
// ExtraResourceAttributes are appended after deployment.environment in
// buildResource, so a caller could overwrite the env tag with "production" or
// "prod-us" and ship spans under an env no dashboard or monitor queries —
// exactly what validate exists to prevent (JIN-1570).
//
// An empty value is skipped: that means no env tag was set at all, which
// Config.validate already refuses before buildResource is reached.
func validateResourceEnvironment(res *sdkresource.Resource) error {
	if res == nil {
		return nil
	}
	for _, kv := range res.Attributes() {
		if !environmentResourceKeys[kv.Key] {
			continue
		}
		value := kv.Value.Emit()
		if value == "" || conventions.IsCanonicalEnvironment(value) {
			continue
		}
		return fmt.Errorf("telemetry: resource %s %q is not canonical; use one of %s, and set it through Config.Environment rather than ExtraResourceAttributes (JIN-1570)",
			kv.Key, value, strings.Join(conventions.Environments, ", "))
	}
	return nil
}

func buildTraceExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if cfg.ExporterTimeout > 0 {
		opts = append(opts, otlptracegrpc.WithTimeout(cfg.ExporterTimeout))
	}
	return otlptracegrpc.New(ctx, opts...)
}

// newBaggageFilter returns a baggagecopy member filter accepting any
// baggage member whose key starts with one of the given prefixes. An
// empty prefix slice rejects every member (useful in tests).
func newBaggageFilter(prefixes []string) baggagecopy.Filter {
	if len(prefixes) == 0 {
		return func(baggage.Member) bool { return false }
	}
	cp := append([]string(nil), prefixes...) // defensive copy
	return func(m baggage.Member) bool {
		k := m.Key()
		for _, p := range cp {
			if strings.HasPrefix(k, p) {
				return true
			}
		}
		return false
	}
}

func noopShutdown(context.Context) error { return nil }
