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

	// SSE: flush per chunk so the IDE sees deltas as they arrive.
	// We flush after every Read regardless of content-type — flushing
	// non-SSE responses costs nothing measurable and keeps the path
	// uniform.
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return fmt.Errorf("stream response: %w", werr)
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
			return fmt.Errorf("stream response: %w", rerr)
		}
	}
}
