package conventions_test

import (
	"testing"

	"github.com/jinkoso/jinko-observability/pkg/conventions"

	"github.com/stretchr/testify/assert"
)

func TestIsCanonicalEnvironment_AcceptsCanonicalValues(t *testing.T) {
	for _, env := range []string{
		conventions.EnvDev,
		conventions.EnvSandbox,
		conventions.EnvPreprod,
		conventions.EnvProd,
	} {
		assert.True(t, conventions.IsCanonicalEnvironment(env),
			"%q must be accepted as a canonical env value", env)
	}
}

func TestIsCanonicalEnvironment_RejectsNonCanonicalValues(t *testing.T) {
	// "production"/"staging" are the legacy spellings; "prod-us"/"prod-eu" put
	// region into env, which JIN-1570 moves to host/cluster tags. Case and
	// whitespace variants are rejected too — Datadog tag values are matched
	// literally.
	for _, env := range []string{
		"", "production", "staging", "development", "test", "local",
		"prod-us", "prod-eu", "dev-us", "PROD", "Prod", " prod", "prod ",
	} {
		assert.False(t, conventions.IsCanonicalEnvironment(env),
			"%q must be rejected as a non-canonical env value", env)
	}
}

func TestEnvironments_ListsExactlyTheCanonicalSet(t *testing.T) {
	// Guards against the slice and the predicate drifting apart: the slice is
	// what error messages show users, the predicate is what accepts their input.
	assert.Equal(t,
		[]string{"dev", "sandbox", "preprod", "prod"},
		conventions.Environments)

	for _, env := range conventions.Environments {
		assert.True(t, conventions.IsCanonicalEnvironment(env),
			"Environments lists %q but IsCanonicalEnvironment rejects it", env)
	}
}
