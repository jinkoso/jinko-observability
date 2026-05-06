package conventions

// HTTP headers carrying request-scoped identity and correlation data.
//
// These headers are read by the inbound RequestContext middleware and
// populated by upstream services (jinko-mcp, jinko-app-web, jinko-cli)
// when calling downstream services.
//
// Identity headers (X-User-ID, X-Tenant-ID, X-Session-ID) are accepted
// only on internal trust boundaries. Public-facing entrypoints derive
// identity from authenticated credentials (JWT, API key) instead.
const (
	HeaderRequestID    = "X-Request-ID"
	HeaderTenantID     = "X-Tenant-ID"
	HeaderUserID       = "X-User-ID"
	HeaderSessionID    = "X-Session-ID"
	HeaderLocale       = "X-Locale"
	HeaderCountry      = "X-Country"
	HeaderCity         = "X-City"
	HeaderSource       = "X-Source"
	HeaderPlatform     = "X-Platform"
	HeaderJinkoCaller  = "X-Jinko-Caller"
	HeaderClientIP     = "X-Real-IP"
	HeaderForwardedFor = "X-Forwarded-For"
	HeaderCFConnecting = "CF-Connecting-IP"

	// Originating-client metadata (v0.3.0). Forwarded by jinko-mcp on
	// behalf of the actual MCP client (chatgpt | claude | …). Distinct
	// from HeaderJinkoCaller, which identifies the *internal service*
	// making the call (mcp-bff, jinko-cli, …).
	HeaderClientKind     = "X-Client-Kind"
	HeaderClientVersion  = "X-Client-Version"
	HeaderOrganizationID = "X-Organization-ID"
	HeaderConversationID = "X-Conversation-ID"
)
