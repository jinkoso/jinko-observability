package telemetry

import (
	"context"
	"strings"
	"testing"

	"github.com/jinkoso/jinko-observability/pkg/conventions"

	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

// resourceAttr returns the value of key in res, and whether it is set.
func resourceAttr(res *sdkresource.Resource, key attribute.Key) (string, bool) {
	for _, kv := range res.Attributes() {
		if kv.Key == key {
			return kv.Value.Emit(), true
		}
	}
	return "", false
}

// JIN-1570: Config.validate checks cfg.Environment, but buildResource appends
// ExtraResourceAttributes AFTER deployment.environment — so a caller could set
// a canonical Environment, pass validate, and then overwrite the env tag with
// "production" on the resource that actually ships. The merged resource is
// re-checked, so the override is refused instead.
//
// Pure buildResource test: no exporter, no global TracerProvider.
func TestBuildResource_RejectsNonCanonicalEnvFromExtraResourceAttributes(t *testing.T) {
	cases := []struct {
		name      string
		extra     sdkresource.Option
		offending string
	}{
		{
			name:      "deployment.environment override",
			extra:     sdkresource.WithAttributes(attribute.String("deployment.environment", "production")),
			offending: "production",
		},
		{
			name:      "region-qualified value",
			extra:     sdkresource.WithAttributes(attribute.String("deployment.environment", "prod-us")),
			offending: "prod-us",
		},
		{
			name:      "datadog env tag set directly",
			extra:     sdkresource.WithAttributes(attribute.String("env", "staging")),
			offending: "staging",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildResource(context.Background(), Config{
				ServiceName:             "test",
				Environment:             conventions.EnvDev, // canonical: only the override is wrong
				ExtraResourceAttributes: []sdkresource.Option{tc.extra},
			})
			if err == nil {
				t.Fatalf("a non-canonical env via %s must be refused, got nil error", tc.name)
			}
			msg := err.Error()
			if !strings.Contains(msg, "not canonical") {
				t.Errorf("error must say the value is not canonical; got: %v", err)
			}
			if !strings.Contains(msg, tc.offending) {
				t.Errorf("error must name the offending value %q; got: %v", tc.offending, err)
			}
			if !strings.Contains(msg, strings.Join(conventions.Environments, ", ")) {
				t.Errorf("error must list the canonical vocabulary; got: %v", err)
			}
		})
	}
}

// The default path: env tag straight from the validated Config.
func TestBuildResource_CanonicalConfigEnvironment(t *testing.T) {
	for _, env := range conventions.Environments {
		t.Run(env, func(t *testing.T) {
			res, err := buildResource(context.Background(), Config{
				ServiceName: "test",
				Environment: env,
			})
			if err != nil {
				t.Fatalf("canonical environment %q must be accepted, got: %v", env, err)
			}
			if got, _ := resourceAttr(res, "deployment.environment"); got != env {
				t.Errorf("deployment.environment = %q, want %q", got, env)
			}
		})
	}
}

// A Config with no Environment reaches buildResource only in tests —
// Config.validate refuses it first. buildResource must not double-report it as
// a non-canonical resource attribute, because the attribute is never written.
func TestBuildResource_NoEnvironmentIsNotReportedHere(t *testing.T) {
	res, err := buildResource(context.Background(), Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("buildResource must leave the empty-Environment case to Config.validate, got: %v", err)
	}
	if got, ok := resourceAttr(res, "deployment.environment"); ok {
		t.Errorf("deployment.environment = %q, want it absent", got)
	}
}

// deploymentEnvironmentNameKey is the newer OTel semantic-convention spelling
// of the env attribute (semconv >= v1.27). This module pins v1.26.0, which has
// no constant for it, but the Datadog OTLP intake maps it onto the same `env`
// tag — so a caller can retag a resource through it just as effectively as
// through deployment.environment, and the merged-resource check must cover it.
const deploymentEnvironmentNameKey = attribute.Key("deployment.environment.name")

// JIN-1570 (Codex follow-up): validateResourceEnvironment used to `continue`
// on an empty value, on the reasoning that an absent env tag is
// Config.validate's problem. But ExtraResourceAttributes are appended AFTER
// deployment.environment, so deployment.environment="" does not leave the
// attribute absent — it overwrites the validated value with an empty string,
// and the skip let that through. The resource then ships spans tagged env:"" ,
// which is exactly what Config.validate refuses before buildResource is
// reached.
func TestBuildResource_RejectsEmptyEnvFromExtraResourceAttributes(t *testing.T) {
	cases := []struct {
		name string
		key  attribute.Key
	}{
		{"deployment.environment blanked", "deployment.environment"},
		{"datadog env tag blanked", "env"},
		{"newer semconv key blanked", deploymentEnvironmentNameKey},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildResource(context.Background(), Config{
				ServiceName: "test",
				Environment: conventions.EnvDev, // canonical: only the override is wrong
				ExtraResourceAttributes: []sdkresource.Option{
					sdkresource.WithAttributes(attribute.String(string(tc.key), "")),
				},
			})
			if err == nil {
				t.Fatalf("%s blanks the env tag and must be refused, got nil error", tc.key)
			}
			msg := err.Error()
			if !strings.Contains(msg, string(tc.key)+` ""`) {
				t.Errorf("error must name the blanked attribute %s and show its empty value; got: %v", tc.key, err)
			}
			if !strings.Contains(msg, "not canonical") {
				t.Errorf("error must say the value is not canonical; got: %v", err)
			}
		})
	}
}

// The newer semconv spelling is a second door onto the same Datadog `env`
// tag, so a non-canonical value through it is refused exactly like the v1.26
// key — otherwise the check is one attribute rename away from being bypassed.
func TestBuildResource_RejectsNonCanonicalDeploymentEnvironmentName(t *testing.T) {
	for _, offending := range []string{"production", "prod-us", "staging"} {
		t.Run(offending, func(t *testing.T) {
			_, err := buildResource(context.Background(), Config{
				ServiceName: "test",
				Environment: conventions.EnvDev,
				ExtraResourceAttributes: []sdkresource.Option{
					sdkresource.WithAttributes(
						attribute.String(string(deploymentEnvironmentNameKey), offending),
					),
				},
			})
			if err == nil {
				t.Fatalf("deployment.environment.name=%s must be refused, got nil error", offending)
			}
			msg := err.Error()
			if !strings.Contains(msg, offending) {
				t.Errorf("error must name the offending value %q; got: %v", offending, err)
			}
			if !strings.Contains(msg, string(deploymentEnvironmentNameKey)) {
				t.Errorf("error must name the attribute it refused; got: %v", err)
			}
		})
	}
}

// JIN-1570 (Codex follow-up): membership in the canonical vocabulary is not
// enough — the value has to be THE configured one. A prod caller with
// Environment: "prod" and an ExtraResourceAttributes override of env="dev"
// passed the vocabulary check, because "dev" is canonical, and then shipped
// production spans under env:dev. The merged resource can also end up
// self-contradictory (deployment.environment="prod" next to env="dev"), which
// is the same mis-tagging the check exists to prevent.
func TestBuildResource_RejectsCanonicalEnvThatDisagreesWithConfig(t *testing.T) {
	const configured = conventions.EnvProd

	cases := []struct {
		name     string
		key      attribute.Key
		override string
	}{
		{"datadog env tag disagrees", "env", conventions.EnvDev},
		{"deployment.environment disagrees", "deployment.environment", conventions.EnvSandbox},
		{"newer semconv key disagrees", deploymentEnvironmentNameKey, conventions.EnvPreprod},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildResource(context.Background(), Config{
				ServiceName: "test",
				Environment: configured,
				ExtraResourceAttributes: []sdkresource.Option{
					sdkresource.WithAttributes(attribute.String(string(tc.key), tc.override)),
				},
			})
			if err == nil {
				t.Fatalf("%s=%q contradicts Config.Environment %q and must be refused, got nil error",
					tc.key, tc.override, configured)
			}
			msg := err.Error()
			if !strings.Contains(msg, string(tc.key)) {
				t.Errorf("error must name the attribute it refused (%s); got: %v", tc.key, err)
			}
			if !strings.Contains(msg, `"`+tc.override+`"`) {
				t.Errorf("error must name the overriding value %q; got: %v", tc.override, err)
			}
			if !strings.Contains(msg, `"`+configured+`"`) {
				t.Errorf("error must name the configured value %q so both sides of the disagreement are visible; got: %v", configured, err)
			}
		})
	}
}

// The rule is agreement, not a ban: an override that restates the configured
// env is accepted and lands on the merged resource, and unrelated overrides
// are untouched.
func TestBuildResource_AcceptsEnvOverrideThatAgreesWithConfig(t *testing.T) {
	res, err := buildResource(context.Background(), Config{
		ServiceName: "test",
		Environment: conventions.EnvPreprod,
		ExtraResourceAttributes: []sdkresource.Option{
			sdkresource.WithAttributes(
				attribute.String("deployment.environment", conventions.EnvPreprod),
				attribute.String(string(deploymentEnvironmentNameKey), conventions.EnvPreprod),
				attribute.String("env", conventions.EnvPreprod),
				attribute.String("service.namespace", "jinko"),
			),
		},
	})
	if err != nil {
		t.Fatalf("env attributes that agree with Config.Environment must be accepted, got: %v", err)
	}
	for _, key := range []attribute.Key{"deployment.environment", deploymentEnvironmentNameKey, "env"} {
		if got, ok := resourceAttr(res, key); !ok || got != conventions.EnvPreprod {
			t.Errorf("%s = %q (set=%v), want %q", key, got, ok, conventions.EnvPreprod)
		}
	}
	if got, ok := resourceAttr(res, "service.namespace"); !ok || got != "jinko" {
		t.Errorf("service.namespace = %q (set=%v), want %q — unrelated overrides must be untouched", got, ok, "jinko")
	}
}
