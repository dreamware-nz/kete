// Package ccproxy is the cc-proxy upstream adapter.
//
// cc-proxy (dreamware-nz/cc-proxy) is wire-identical to Anthropic-direct
// from kete's perspective: same body, same SSE, same prompt-cache
// semantics. Only the base URL differs (default http://127.0.0.1:8787)
// and inbound auth uses cc-proxy's shared-secret x-api-key.
//
// We could just reuse anthropic.Adapter with a different BaseURL —
// and we do, more or less — but having the type live in its own
// package keeps the upstream-selector code obvious to read.
package ccproxy

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/dreamware-nz/kete/internal/adapter/anthropic"
)

const defaultBaseURL = "http://127.0.0.1:8787"

// New builds a cc-proxy adapter. KETE_CC_PROXY_URL overrides the URL.
// KETE_CC_PROXY_KEY is a sanity-check: cc-proxy will reject without
// it, so we surface a clear error early rather than waiting for a
// 401 to come back through the proxy.
func New() (*anthropic.Adapter, error) {
	url := defaultBaseURL
	if v := os.Getenv("KETE_CC_PROXY_URL"); v != "" {
		url = v
	}
	if os.Getenv("KETE_CC_PROXY_KEY") == "" {
		return nil, errors.New("ccproxy: KETE_CC_PROXY_KEY not set; cc-proxy will reject")
	}
	// cc-proxy's wire is anthropic-shaped; reuse the adapter with the
	// adjusted URL. Naming via a wrapper would just hide this; the
	// "anthropic" name on the wire is honest.
	return &anthropic.Adapter{
		BaseURL:    url,
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

// silence unused-import grumble; ctx-typed signature on this package's
// public ctor would be premature.
var _ = context.Background
