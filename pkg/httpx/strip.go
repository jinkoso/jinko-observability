package httpx

import "net/http"

// baggageStripper is an http.RoundTripper that removes the W3C
// "baggage" header from every outbound request before delegating to a
// wrapped RoundTripper. It is composed inside NewExternalClient so
// callers cannot accidentally leak identity baggage to third parties.
//
// The trace context headers (traceparent, tracestate) are left intact
// so vendor calls remain visible nested inside the parent APM trace.
type baggageStripper struct {
	base http.RoundTripper
}

func (b *baggageStripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone to avoid mutating the caller's request struct.
	out := req.Clone(req.Context())
	out.Header.Del("baggage")
	return b.base.RoundTrip(out)
}
