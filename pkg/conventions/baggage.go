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
	BaggageUserID       = "user.id"
	BaggageTenantID     = "tenant.id"
	BaggageSessionID    = "session.id"
	BaggageMCPSessionID = "mcp.session.id"
	BaggageRequestID    = "request.id"
)

// BaggageKeyPrefixes lists the namespaces the baggagecopy SpanProcessor
// promotes to span attributes. Keys outside these prefixes are still
// transported via the baggage header but do not appear on spans.
//
// Add a prefix here only after weighing cardinality and PII risk.
var BaggageKeyPrefixes = []string{
	"user.",
	"tenant.",
	"session.",
	"mcp.",
	"request.",
}
