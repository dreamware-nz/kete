package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// applyMiddlewareChain wraps h with the proxy's request-side
// middleware so we can assert per-middleware behaviour without spinning
// up the full chi tree.
func applyMiddlewareChain(h http.Handler, max int64, timeout time.Duration) http.Handler {
	if timeout > 0 {
		h = timeoutMiddleware(timeout)(h)
	}
	if max > 0 {
		h = maxBodyMiddleware(max)(h)
	}
	return h
}

func TestMaxBody_Returns413(t *testing.T) {
	const cap = 1024
	h := applyMiddlewareChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b bytes.Buffer
		_, err := io.Copy(&b, r.Body)
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
	}), cap, 0)

	body := bytes.Repeat([]byte("a"), cap+1)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status=%d, want 413", rr.Code)
	}
}

func TestMaxBody_AllowsUnderLimit(t *testing.T) {
	const cap = 1024
	h := applyMiddlewareChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(200)
	}), cap, 0)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(bytes.Repeat([]byte("a"), 100)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("status=%d, want 200", rr.Code)
	}
}

// TestTimeout_FiresOnSlowDownstream asserts the per-request context
// gets cancelled after the configured deadline. The handler simulates
// a slow upstream by waiting on its context.
func TestTimeout_FiresOnSlowDownstream(t *testing.T) {
	h := applyMiddlewareChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			if !errors.Is(r.Context().Err(), context.DeadlineExceeded) {
				t.Errorf("ctx err=%v, want DeadlineExceeded", r.Context().Err())
			}
			w.WriteHeader(http.StatusGatewayTimeout)
		case <-time.After(time.Second):
			t.Fatal("ctx never cancelled")
		}
	}), 0, 50*time.Millisecond)

	t0 := time.Now()
	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(""))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if elapsed := time.Since(t0); elapsed > 200*time.Millisecond {
		t.Errorf("took %s, want <= 200 ms", elapsed)
	}
}
