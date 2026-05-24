package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestForward_ByteExact_RequestBody asserts that the request bytes the
// upstream receives are identical to the bytes the client sent. This
// is the load-bearing invariant for prompt-cache compatibility.
func TestForward_ByteExact_RequestBody(t *testing.T) {
	wireIn := []byte(`{"model":"claude","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	want := sha256.Sum256(wireIn)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ := io.ReadAll(r.Body)
		if h := sha256.Sum256(got); !bytes.Equal(h[:], want[:]) {
			t.Errorf("upstream body sha=%s want=%s\n got=%q\nwant=%q",
				hex.EncodeToString(h[:8]), hex.EncodeToString(want[:8]), got, wireIn)
		}
		// Echo a trivial 200 so the client side is happy.
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	rr := httptest.NewRecorder()
	clientHdr := http.Header{
		"X-Api-Key":      []string{"sk-test"},
		"Content-Type":   []string{"application/json"},
		"User-Agent":     []string{"crush/0.1"}, // must NOT pass through
	}
	if err := forward(
		context.Background(),
		http.DefaultClient,
		http.MethodPost, upstream.URL+"/v1/messages",
		wireIn, clientHdr, rr,
	); err != nil {
		t.Fatalf("forward: %v", err)
	}
	if rr.Code != 200 {
		t.Errorf("status=%d, want 200", rr.Code)
	}
	if got := rr.Body.String(); got != `{"ok":true}` {
		t.Errorf("body=%q", got)
	}
}

func TestForward_FiltersHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			t.Errorf("Cookie leaked: %q", r.Header.Get("Cookie"))
		}
		if r.Header.Get("User-Agent") != "Go-http-client/1.1" && r.Header.Get("User-Agent") != "" {
			// net/http auto-adds its own UA; that's fine. We just need
			// the *client's* UA to not propagate.
			if r.Header.Get("User-Agent") == "crush/0.1" {
				t.Errorf("crush UA leaked")
			}
		}
		if r.Header.Get("X-Api-Key") != "sk-test" {
			t.Errorf("x-api-key=%q, want sk-test", r.Header.Get("X-Api-Key"))
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()
	rr := httptest.NewRecorder()
	hdr := http.Header{
		"X-Api-Key":  []string{"sk-test"},
		"Cookie":     []string{"forbidden"},
		"User-Agent": []string{"crush/0.1"},
	}
	if err := forward(context.Background(), http.DefaultClient, http.MethodPost,
		upstream.URL, []byte("body"), hdr, rr); err != nil {
		t.Fatal(err)
	}
}
