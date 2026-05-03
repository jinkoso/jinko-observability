// Package conventions defines the single source of truth for cross-service
// observability conventions used across Jinko services: HTTP header names,
// W3C baggage keys, span attribute keys, and well-known service names.
//
// All other packages in this module reference these constants instead of
// hard-coding strings, so a rename is a single-file change.
//
// Stability: Experimental. The API may change without notice during the
// v0.2.x line. Pin to a tagged release.
package conventions
