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
	// attribute traffic to the authentication method / key / plane. The org
	// is carried by the existing BaggageOrganizationID ("organization.id")
	// and the acting end user by the existing BaggageUserID ("user.id") —
	// there are no separate org.id / enduser.id keys. Carry no PII (IDs only).
	BaggageAuthMethod = "auth.method"
	BaggageAuthKeyID  = "auth.key_id"
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
// `auth.` was added for JIN-1176 to carry the identity-plane auth attributes
// (auth method / key id / plane). The org is already promoted via
// `organization.` and the acting end user via `user.` — there are no separate
// `org.` / `enduser.` prefixes. All low cardinality and PII-free (IDs only).
var BaggageKeyPrefixes = []string{
	"user.",
	"tenant.",
	"session.",
	"mcp.",
	"request.",
	"client.",
	"organization.",
	"auth.",
}
