package telemetry

import (
	"context"

	"github.com/jinkoso/jinko-observability/pkg/conventions"

	"go.opentelemetry.io/otel/baggage"
)

// Identity is the standard cross-service identity bag carried in W3C
// baggage. Set it once at the trust boundary (after authentication);
// every downstream service reads it via ReadIdentity.
//
// Empty fields are skipped — a baggage member is only created for
// fields with a non-empty value.
type Identity struct {
	UserID       string
	TenantID     string
	SessionID    string
	MCPSessionID string
	RequestID    string
}

// SetIdentity stores the given identity as W3C baggage on ctx.
//
// SetIdentity merges with any baggage already present: existing members
// remain, and members for matching identity keys are overwritten. The
// returned context replaces the input — callers must use it.
//
// Empty identity fields are not written. Pass non-empty values only
// when they have been validated against an authenticated subject.
func SetIdentity(ctx context.Context, id Identity) context.Context {
	b := baggage.FromContext(ctx)

	pairs := [...]struct {
		key, value string
	}{
		{conventions.BaggageUserID, id.UserID},
		{conventions.BaggageTenantID, id.TenantID},
		{conventions.BaggageSessionID, id.SessionID},
		{conventions.BaggageMCPSessionID, id.MCPSessionID},
		{conventions.BaggageRequestID, id.RequestID},
	}

	for _, p := range pairs {
		if p.value == "" {
			continue
		}
		m, err := baggage.NewMemberRaw(p.key, p.value)
		if err != nil {
			// NewMemberRaw rejects keys/values that violate W3C charset.
			// Skip the bad member rather than failing the request.
			continue
		}
		nb, err := b.SetMember(m)
		if err != nil {
			continue
		}
		b = nb
	}

	return baggage.ContextWithBaggage(ctx, b)
}

// ReadIdentity extracts the identity stored in ctx's baggage. Missing
// members surface as empty strings — callers decide whether that
// constitutes an error in their context.
func ReadIdentity(ctx context.Context) Identity {
	b := baggage.FromContext(ctx)
	return Identity{
		UserID:       b.Member(conventions.BaggageUserID).Value(),
		TenantID:     b.Member(conventions.BaggageTenantID).Value(),
		SessionID:    b.Member(conventions.BaggageSessionID).Value(),
		MCPSessionID: b.Member(conventions.BaggageMCPSessionID).Value(),
		RequestID:    b.Member(conventions.BaggageRequestID).Value(),
	}
}

// StripBaggage removes ALL baggage from ctx. Use this when the trust
// boundary changes — e.g. before invoking a third-party API client
// that reuses the live ctx but should not propagate internal identity.
//
// httpx.NewExternalClient strips baggage from the outbound HTTP header,
// which is the recommended path. Use StripBaggage only when you need
// to drop baggage from the in-process context as well.
func StripBaggage(ctx context.Context) context.Context {
	return baggage.ContextWithBaggage(ctx, baggage.Baggage{})
}
