package conventions

// W3C baggage keys for cross-service identity propagation.
//
// Baggage is set at the trust boundary (after authentication) and read
// by every downstream service for free via the W3C baggage HTTP header.
// The baggagecopy SpanProcessor (see pkg/telemetry) auto-promotes these
// keys onto every span so APM filtering by user/tenant works with no
// per-handler boilerplate.
//
// Do NOT put PII (email, full name, phone) in baggage — baggage flows
// to all outbound HTTP calls, including third-party APIs (Stripe,
// Sabre, etc.). Use NewExternalClient (pkg/httpx) when calling third
// parties to strip baggage at the trust boundary.
const (
	BaggageUserID         = "user.id"
	BaggageTenantID       = "tenant.id"
	BaggageSessionID      = "session.id"
	BaggageMCPSessionID   = "mcp.session.id"
	BaggageRequestID      = "request.id"
	BaggageClientKind     = "client.kind"
	BaggageClientVersion  = "client.version"
	BaggageOrganizationID = "organization.id"

	// JIN-1176 identity-plane keys. Set at the auth trust boundary to
	// attribute traffic to the org, the authentication method/key, and
	// the resolved end user. Carry no PII (IDs only, not emails/names).
	BaggageOrgID      = "org.id"
	BaggageAuthMethod = "auth.method"
	BaggageAuthKeyID  = "auth.key_id"
	BaggageEndUserID  = "enduser.id"
	BaggageAuthPlane  = "auth.plane"
)

// BaggageKeyPrefixes lists the namespaces the baggagecopy SpanProcessor
// promotes to span attributes. Keys outside these prefixes are still
// transported via the baggage header but do not appear on spans.
//
// Add a prefix here only after weighing cardinality and PII risk.
//
// `client.` and `organization.` were added in v0.3.0 to attribute traffic
// to the originating MCP client (chatgpt | claude | claude-code | …) and
// the upstream org/tenant analog. Both have low cardinality (≤10 distinct
// client kinds; org IDs are stable per integrator) and carry no PII.
//
// `org.`, `auth.`, and `enduser.` were added for JIN-1176 to carry the
// identity-plane attributes (org id, auth method/key id/plane, resolved
// end-user id). All low cardinality and PII-free (IDs only).
var BaggageKeyPrefixes = []string{
	"user.",
	"tenant.",
	"session.",
	"mcp.",
	"request.",
	"client.",
	"organization.",
	"org.",
	"auth.",
	"enduser.",
}
