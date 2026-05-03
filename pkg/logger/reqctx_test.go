package logger_test

import (
	"context"
	"testing"

	"github.com/jinkoso/jinko-observability/pkg/logger"
	"github.com/jinkoso/jinko-observability/pkg/telemetry"

	"github.com/stretchr/testify/assert"
)

func fieldMap(fields []logger.Field) map[string]interface{} {
	out := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		out[f.Key] = f.Value
	}
	return out
}

func TestIdentityFields_AllFieldsPresent(t *testing.T) {
	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{
		UserID:       "u_123",
		TenantID:     "acme",
		SessionID:    "s_abc",
		MCPSessionID: "mcp_xyz",
		RequestID:    "req_1",
	})

	got := fieldMap(logger.IdentityFields(ctx))
	assert.Equal(t, "u_123", got["user_id"])
	assert.Equal(t, "acme", got["tenant_id"])
	assert.Equal(t, "s_abc", got["session_id"])
	assert.Equal(t, "mcp_xyz", got["mcp_session_id"])
	assert.Equal(t, "req_1", got["request_id"])
	assert.Len(t, got, 5)
}

func TestIdentityFields_EmptyContext_ReturnsNoFields(t *testing.T) {
	got := logger.IdentityFields(context.Background())
	assert.Empty(t, got, "no baggage -> no fields (don't surface empty-string facets)")
}

func TestIdentityFields_PartialIdentity_OnlyPopulatedFields(t *testing.T) {
	ctx := telemetry.SetIdentity(context.Background(), telemetry.Identity{
		UserID:    "u_only",
		RequestID: "req_only",
	})

	got := fieldMap(logger.IdentityFields(ctx))
	assert.Equal(t, "u_only", got["user_id"])
	assert.Equal(t, "req_only", got["request_id"])
	_, hasTenant := got["tenant_id"]
	_, hasSession := got["session_id"]
	assert.False(t, hasTenant, "missing tenant must not produce a tenant_id field")
	assert.False(t, hasSession, "missing session must not produce a session_id field")
}
