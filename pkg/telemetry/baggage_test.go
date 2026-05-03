package telemetry_test

import (
	"context"
	"testing"

	"github.com/jinkoso/jinko-observability/pkg/conventions"
	"github.com/jinkoso/jinko-observability/pkg/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/baggage"
)

func TestSetIdentity_RoundTrip(t *testing.T) {
	id := telemetry.Identity{
		UserID:       "u_123",
		TenantID:     "acme",
		SessionID:    "s_abc",
		MCPSessionID: "mcp_xyz",
		RequestID:    "req_1",
	}

	ctx := telemetry.SetIdentity(context.Background(), id)
	got := telemetry.ReadIdentity(ctx)

	assert.Equal(t, id, got, "round-trip identity should match")
}

func TestSetIdentity_PersistsInBaggage(t *testing.T) {
	id := telemetry.Identity{UserID: "u_123", TenantID: "acme"}
	ctx := telemetry.SetIdentity(context.Background(), id)

	b := baggage.FromContext(ctx)
	assert.Equal(t, "u_123", b.Member(conventions.BaggageUserID).Value())
	assert.Equal(t, "acme", b.Member(conventions.BaggageTenantID).Value())
}

func TestSetIdentity_EmptyFieldsSkipped(t *testing.T) {
	// Only UserID set; other identity members must remain absent so they
	// don't surface as empty-string facets in Datadog.
	id := telemetry.Identity{UserID: "u_only"}
	ctx := telemetry.SetIdentity(context.Background(), id)

	b := baggage.FromContext(ctx)
	assert.Equal(t, "u_only", b.Member(conventions.BaggageUserID).Value())
	assert.Empty(t, b.Member(conventions.BaggageTenantID).Value())
	assert.Empty(t, b.Member(conventions.BaggageSessionID).Value())
}

func TestSetIdentity_PreservesPriorBaggage(t *testing.T) {
	// Pre-populate with an unrelated entry so we can confirm it survives.
	priorMember, err := baggage.NewMemberRaw("custom.flag", "true")
	require.NoError(t, err)
	prior, err := baggage.New(priorMember)
	require.NoError(t, err)
	ctx := baggage.ContextWithBaggage(context.Background(), prior)

	ctx = telemetry.SetIdentity(ctx, telemetry.Identity{UserID: "u_x"})

	b := baggage.FromContext(ctx)
	assert.Equal(t, "true", b.Member("custom.flag").Value(), "unrelated baggage must not be dropped")
	assert.Equal(t, "u_x", b.Member(conventions.BaggageUserID).Value())
}

func TestSetIdentity_OverwritesMatchingKey(t *testing.T) {
	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{UserID: "u_old"})
	ctx = telemetry.SetIdentity(ctx, telemetry.Identity{UserID: "u_new"})

	got := telemetry.ReadIdentity(ctx)
	assert.Equal(t, "u_new", got.UserID, "second SetIdentity must overwrite")
}

func TestReadIdentity_NoBaggage(t *testing.T) {
	got := telemetry.ReadIdentity(context.Background())
	assert.Equal(t, telemetry.Identity{}, got, "missing baggage should yield zero-value Identity")
}

func TestStripBaggage_RemovesEverything(t *testing.T) {
	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{UserID: "u_x", TenantID: "acme"})
	ctx = telemetry.StripBaggage(ctx)

	b := baggage.FromContext(ctx)
	assert.Equal(t, 0, b.Len(), "StripBaggage must remove all members")
	assert.Equal(t, telemetry.Identity{}, telemetry.ReadIdentity(ctx))
}
