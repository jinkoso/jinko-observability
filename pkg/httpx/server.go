package httpx

import (
	"github.com/gin-gonic/gin"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Middleware returns a Gin middleware that starts an OTel server span
// for every request, extracting the W3C trace context and baggage from
// inbound headers (via the global propagator configured by
// pkg/telemetry.Init).
//
// The span name uses the route template (e.g. "GET /carts/:id") rather
// than the literal URL, to avoid cardinality blow-up in APM resources.
// Routes that miss the template (404s) get a generic name.
func Middleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName,
		otelgin.WithSpanNameFormatter(spanNameFromRoute),
	)
}

func spanNameFromRoute(c *gin.Context) string {
	route := c.FullPath()
	if route == "" {
		// Unmatched route — keep cardinality bounded.
		return c.Request.Method + " <unmatched>"
	}
	return c.Request.Method + " " + route
}
