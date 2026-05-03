package logger

import (
	"context"

	"github.com/jinkoso/jinko-observability/pkg/conventions"
	"github.com/jinkoso/jinko-observability/pkg/telemetry"
)

// IdentityFields returns the standard identity log fields derived from
// the W3C baggage on ctx. Empty members are skipped so a missing tenant
// or user does not surface as `tenant_id=""` (which Datadog renders as
// a real, searchable empty value and creates a useless facet entry).
//
// Use this in handler / service code to enrich a logger:
//
//	log := baseLogger.With(logger.IdentityFields(ctx)...)
//	log.Info(ctx, "cart created", logger.String("cart_id", cartID))
//
// The trace_id / span_id fields are added separately by the Zap core
// (see pkg/logger/trace.go) on every log call.
func IdentityFields(ctx context.Context) []Field {
	id := telemetry.ReadIdentity(ctx)
	fields := make([]Field, 0, 5)
	if id.UserID != "" {
		fields = append(fields, String(conventions.LogFieldUserID, id.UserID))
	}
	if id.TenantID != "" {
		fields = append(fields, String(conventions.LogFieldTenantID, id.TenantID))
	}
	if id.SessionID != "" {
		fields = append(fields, String(conventions.LogFieldSessionID, id.SessionID))
	}
	if id.MCPSessionID != "" {
		fields = append(fields, String(conventions.LogFieldMCPSessionID, id.MCPSessionID))
	}
	if id.RequestID != "" {
		fields = append(fields, String(conventions.LogFieldRequestID, id.RequestID))
	}
	return fields
}
