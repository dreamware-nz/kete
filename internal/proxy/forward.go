package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// forward sends rawBody to upstreamURL using the sanitised headers and
// streams the response back to w. Byte-exact: the request that hits
// the upstream is identical to what was POSTed to us, modulo the
// header whitelist (ADR 0006).
//
// We deliberately do not use httputil.ReverseProxy: it buffers, mutates
// some headers, and gives us no clean place to splice byte-level
// edits. ~80 lines of net/http here is honest.
func forward(
	ctx context.Context,
	httpClient *http.Client,
	method, upstreamURL string,
	rawBody []byte,
	clientHeaders http.Header,
	w http.ResponseWriter,
) error {
	req, err := http.NewRequestWithContext(ctx, method, upstreamURL, bytes.NewReader(rawBody))
	if err != nil {
		return fmt.Errorf("build upstream req: %w", err)
	}
	// ContentLength must be set explicitly when using a non-nil Body
	// reader so the upstream sees the right size; net/http would
	// otherwise mark it -1 and chunk-encode, which mutates the wire
	// shape and could affect cache prefix.
	req.ContentLength = int64(len(rawBody))

	for k, vs := range SanitiseHeaders(clientHeaders) {
		req.Header[k] = vs
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("forward: %w", err)
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		w.Header()[k] = vs
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stream response: %w", err)
	}
	return nil
}
