package keepalive

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestKeepalive_FiresAfterIdleThreshold drives tickAt to exercise the
// idle-detection path without sleeping for real minutes.
func TestKeepalive_FiresAfterIdleThreshold(t *testing.T) {
	var hits atomic.Int32
	var lastBody atomic.Pointer[[]byte]
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		lastBody.Store(&body)
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	m := NewManager()
	m.client = upstream.Client()

	original := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	hdr := http.Header{"Content-Type": []string{"application/json"}}

	// Stash with lastForward = "5 minutes ago" so a tick "now" sees
	// the session idle past threshold.
	m.Stash("session-1", original, hdr, upstream.URL)
	m.mu.Lock()
	m.sessions["session-1"].lastForward = time.Now().Add(-5 * time.Minute)
	m.mu.Unlock()

	m.tickAt(time.Now())

	// fire() runs in a goroutine; wait for the upstream to record the
	// hit.
	deadline := time.Now().Add(time.Second)
	for hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits=%d, want 1", hits.Load())
	}
	body := *lastBody.Load()
	if !bytes.Contains(body, []byte(`"role":"user","content":"."`)) {
		t.Errorf("upstream body missing keepalive payload:\n%s", body)
	}
}

func TestKeepalive_RespectsCap(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	m := NewManager()
	m.client = upstream.Client()
	m.Stash("s", []byte(`{"messages":[]}`), http.Header{}, upstream.URL)

	// Mark idle, fire 3 ticks; only 2 should result in keepalive++.
	m.mu.Lock()
	m.sessions["s"].lastForward = time.Now().Add(-5 * time.Minute)
	m.mu.Unlock()
	for i := 0; i < 3; i++ {
		m.tickAt(time.Now())
	}
	m.mu.Lock()
	got := m.sessions["s"].keepalives
	m.mu.Unlock()
	if got != maxPerIdlePeriod {
		t.Errorf("keepalives=%d, want %d", got, maxPerIdlePeriod)
	}
}

func TestKeepalive_ExpiresAfterMaxIdleTotal(t *testing.T) {
	m := NewManager()
	m.Stash("s", []byte(`{"messages":[]}`), http.Header{}, "http://nowhere")
	m.mu.Lock()
	m.sessions["s"].lastForward = time.Now().Add(-15 * time.Minute)
	m.mu.Unlock()
	m.tickAt(time.Now())
	m.mu.Lock()
	_, present := m.sessions["s"]
	m.mu.Unlock()
	if present {
		t.Error("session should have been evicted past maxIdleTotal")
	}
}
