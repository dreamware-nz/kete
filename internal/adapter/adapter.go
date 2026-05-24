// Package adapter is the per-vendor wire implementation for kete's
// proxy. ADR 0007 splits the surface into Wire (byte-level forwarding)
// and Semantics (typed view); only Wire ships in plan 002 because
// that's the only interface a second caller needs. Semantics methods
// will land alongside the plans that need them (extraction in plan 011,
// injection ranking in plan 010, drift in plan 007).
package adapter

import (
	"context"
	"net/http"
)

// Wire is the per-vendor HTTP forwarding surface. Anthropic-direct,
// cc-proxy, and Bedrock each ship one. The orchestrator depends on
// this interface; vendor packages provide implementations.
type Wire interface {
	// Forward sends rawBody to the configured upstream and streams the
	// response to w. Implementations must preserve byte-exactness on
	// the request (ADR 0006); only Bedrock is allowed to deviate
	// (ADR 0014).
	Forward(ctx context.Context, rawBody []byte, headers http.Header, w http.ResponseWriter) error

	// Name returns the upstream identifier ("anthropic", "cc-proxy",
	// "bedrock") for logs and selection.
	Name() string
}
