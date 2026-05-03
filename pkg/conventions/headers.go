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
	HeaderSource       = "X-Source"
	HeaderPlatform     = "X-Platform"
	HeaderJinkoCaller  = "X-Jinko-Caller"
	HeaderClientIP     = "X-Real-IP"
	HeaderForwardedFor = "X-Forwarded-For"
	HeaderCFConnecting = "CF-Connecting-IP"
)
