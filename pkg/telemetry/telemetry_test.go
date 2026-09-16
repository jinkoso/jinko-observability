package telemetry_test

import (
	"context"
	"testing"

	"github.com/jinkoso/jinko-observability/pkg/conventions"
	"github.com/jinkoso/jinko-observability/pkg/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
)

func TestInit_DisabledIsNoop(t *testing.T) {
	shutdown, err := telemetry.Init(context.Background(), telemetry.Config{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// No-op shutdown must not error and must be idempotent.
	assert.NoError(t, shutdown(context.Background()))
	assert.NoError(t, shutdown(context.Background()))
}

func TestInit_DisabledStillRegistersPropagator(t *testing.T) {
	// A disabled service must still propagate the W3C baggage (and
	// traceparent) header on its outbound calls — otherwise cross-service
	// tracing context is silently dropped at the boundary. Init registers
	// the composite propagator unconditionally; only the exporter /
	// TracerProvider / sampler are gated on Enabled.
	_, err := telemetry.Init(context.Background(), telemetry.Config{Enabled: false})
	require.NoError(t, err)

	prop := otel.GetTextMapPropagator()
	require.NotNil(t, prop)

	// Seed a context with a baggage member and inject it into a carrier.
	member, err := baggage.NewMemberRaw("tenant.id", "acme")
	require.NoError(t, err)
	b, err := baggage.New(member)
	require.NoError(t, err)
	ctx := baggage.ContextWithBaggage(context.Background(), b)

	carrier := propagation.MapCarrier{}
	prop.Inject(ctx, carrier)

	require.Contains(t, carrier, "baggage", "disabled Init must still inject the W3C baggage header")
	assert.Contains(t, carrier["baggage"], "tenant.id=acme")

	// And the same propagator must round-trip it back out via Extract.
	extracted := baggage.FromContext(prop.Extract(context.Background(), carrier))
	assert.Equal(t, "acme", extracted.Member("tenant.id").Value())
}

func TestInit_EnabledRequiresServiceName(t *testing.T) {
	_, err := telemetry.Init(context.Background(), telemetry.Config{
		Enabled:      true,
		Environment:  conventions.EnvDev,
		OTLPEndpoint: "localhost:4317",
		Insecure:     true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ServiceName")
}

func TestInit_EnabledRequiresEnvironment(t *testing.T) {
	// Datadog Unified Service Tagging requires `env`. Without it, prod
	// traces silently land in the same APM bucket as preprod/dev, which
	// is far worse than failing fast at startup.
	_, err := telemetry.Init(context.Background(), telemetry.Config{
		Enabled:      true,
		ServiceName:  "test",
		OTLPEndpoint: "localhost:4317",
		Insecure:     true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Environment")
}

func TestInit_EnabledRequiresOTLPEndpoint(t *testing.T) {
	_, err := telemetry.Init(context.Background(), telemetry.Config{
		Enabled:     true,
		ServiceName: "test",
		Environment: conventions.EnvDev,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OTLPEndpoint")
}

func TestInit_EnabledRejectsNonCanonicalEnvironment(t *testing.T) {
	// JIN-1570: dev / sandbox / preprod / prod is the whole estate vocabulary.
	// Anything else ships spans under an env no dashboard or monitor queries,
	// so validation must fail at startup rather than at investigation time.
	// Every config below is otherwise complete — only Environment is wrong.
	for _, env := range []string{"production", "prod-us", "staging", "PROD"} {
		t.Run(env, func(t *testing.T) {
			_, err := telemetry.Init(context.Background(), telemetry.Config{
				Enabled:      true,
				ServiceName:  "test",
				Environment:  env,
				OTLPEndpoint: "localhost:4317",
				Insecure:     true,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not canonical")
			assert.Contains(t, err.Error(), env)
			assert.Contains(t, err.Error(), "dev, sandbox, preprod, prod")
		})
	}
}

func TestInit_EnabledAcceptsCanonicalEnvironments(t *testing.T) {
	// Validation only. OTLPEndpoint is left empty on purpose so Init fails at
	// the next check and never builds an exporter — same trick as
	// TestInit_EnabledRequiresOTLPEndpoint, which keeps the test off the
	// network and out of the global TracerProvider.
	for _, env := range conventions.Environments {
		t.Run(env, func(t *testing.T) {
			_, err := telemetry.Init(context.Background(), telemetry.Config{
				Enabled:     true,
				ServiceName: "test",
				Environment: env,
			})
			require.Error(t, err, "expected the OTLPEndpoint check to be reached")
			assert.Contains(t, err.Error(), "OTLPEndpoint",
				"canonical env %q must pass validation", env)
			assert.NotContains(t, err.Error(), "canonical",
				"canonical env %q must not be rejected as non-canonical", env)
		})
	}
}
