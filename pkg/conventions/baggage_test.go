package conventions_test

import (
	"testing"

	"github.com/jinkoso/jinko-observability/pkg/conventions"

	"github.com/stretchr/testify/assert"
)

func TestBaggageKeyPrefixes_IncludeIdentityPlane(t *testing.T) {
	// JIN-1176 added the identity-plane `auth.` namespace. It must be present
	// so the baggagecopy SpanProcessor promotes auth.* baggage onto spans for
	// APM filtering. (org → existing `organization.`, end user → existing `user.`.)
	for _, prefix := range []string{"auth."} {
		assert.Contains(t, conventions.BaggageKeyPrefixes, prefix,
			"BaggageKeyPrefixes must include %q", prefix)
	}
}

func TestBaggageKeyPrefixes_RetainsExisting(t *testing.T) {
	// Guard against accidentally dropping a previously-promoted namespace.
	for _, prefix := range []string{
		"user.", "tenant.", "session.", "mcp.", "request.", "client.", "organization.",
	} {
		assert.Contains(t, conventions.BaggageKeyPrefixes, prefix,
			"BaggageKeyPrefixes must still include %q", prefix)
	}
}

func TestIdentityPlaneBaggageKeys(t *testing.T) {
	// Each identity-plane key must fall under one of the promoted prefixes,
	// otherwise the value would transport but never surface on a span.
	cases := map[string]string{
		conventions.BaggageAuthMethod: "auth.method",
		conventions.BaggageAuthKeyID:  "auth.key_id",
		conventions.BaggageAuthPlane:  "auth.plane",
	}
	for got, want := range cases {
		assert.Equal(t, want, got)
	}
}
