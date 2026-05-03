package conventions

// Span and log attribute keys used across services. These mirror the
// baggage keys (without the "Baggage" prefix in the constant name) and
// align with OpenTelemetry semantic conventions where possible.
//
// Identity attributes are written by the baggagecopy SpanProcessor —
// handlers should NOT set them manually. Business attributes (cart.id,
// offer.id, etc.) are set by handler code with span.SetAttributes.
const (
	// Identity (auto-promoted from baggage)
	AttrUserID       = "user.id"
	AttrTenantID     = "tenant.id"
	AttrSessionID    = "session.id"
	AttrMCPSessionID = "mcp.session.id"
	AttrRequestID    = "request.id"

	// Business identifiers (set by handlers)
	AttrCartID     = "cart.id"
	AttrOfferID    = "offer.id"
	AttrQuoteID    = "quote.id"
	AttrBookingID  = "booking.id"
	AttrProviderID = "provider.id"
)

// Log field keys (snake_case, JSON-friendly). Distinct from span
// attribute keys because logs follow service-conventional snake_case.
const (
	LogFieldUserID       = "user_id"
	LogFieldTenantID     = "tenant_id"
	LogFieldSessionID    = "session_id"
	LogFieldMCPSessionID = "mcp_session_id"
	LogFieldRequestID    = "request_id"
)
