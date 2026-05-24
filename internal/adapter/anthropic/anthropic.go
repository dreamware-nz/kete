// Package anthropic is the Anthropic-direct upstream adapter.
package anthropic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// DefaultBaseURL is the public Anthropic API. Override via
// KETE_ANTHROPIC_URL for tests, cc-proxy is a separate adapter.
const DefaultBaseURL = "https://api.anthropic.com"

// Adapter implements adapter.Wire against the Anthropic API.
type Adapter struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New builds an adapter honouring KETE_ANTHROPIC_URL.
func New() *Adapter {
	url := DefaultBaseURL
	if v := os.Getenv("KETE_ANTHROPIC_URL"); v != "" {
		url = v
	}
	return &Adapter{
		BaseURL:    url,
		HTTPClient: http.DefaultClient,
	}
}

// Name reports the upstream id.
func (a *Adapter) Name() string { return "anthropic" }

// Forward proxies the body byte-exact to BaseURL + /v1/messages.
// Header sanitisation stays in the proxy; the adapter trusts what it
// is handed and forwards bytes.
func (a *Adapter) Forward(ctx context.Context, rawBody []byte, headers http.Header, w http.ResponseWriter) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.BaseURL+"/v1/messages", bytes.NewReader(rawBody))
	if err != nil {
		return fmt.Errorf("build req: %w", err)
	}
	req.ContentLength = int64(len(rawBody))
	for k, vs := range headers {
		req.Header[k] = vs
	}

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("forward: %w", err)
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		w.Header()[k] = vs
	}
	w.WriteHeader(resp.StatusCode)

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return fmt.Errorf("write: %w", werr)
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return nil
			}
			if errors.Is(rerr, context.Canceled) {
				return nil
			}
			return fmt.Errorf("read upstream: %w", rerr)
		}
	}
}
