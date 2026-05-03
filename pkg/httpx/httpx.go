// Package httpx provides Jinko-standard HTTP server middleware and HTTP
// clients for inter-service and third-party calls.
//
// The package wraps the OpenTelemetry instrumentation libraries
// (otelgin, otelhttp) with conventions that match the rest of this
// module — sensible defaults, baggage-stripping for external calls,
// and a Gin middleware that pairs with the RequestContext model used
// by all Go services.
//
// Stability: Experimental.
package httpx

import "time"

// Defaults applied by NewTracedClient and NewExternalClient when the
// caller does not override them.
const (
	defaultTimeout             = 30 * time.Second
	defaultMaxIdleConns        = 100
	defaultMaxIdleConnsPerHost = 10
	defaultIdleConnTimeout     = 90 * time.Second
)
