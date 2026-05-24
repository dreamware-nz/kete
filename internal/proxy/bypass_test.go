package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dreamware-nz/kete/internal/adapter/anthropic"
	"github.com/dreamware-nz/kete/internal/extract"
	"github.com/dreamware-nz/kete/internal/store"
)

// TestBypassHeader_SkipsCaptureAndInject confirms that requests
// carrying extract.BypassHeader are forwarded straight through —
// no capture row written, no memory injected. Required to break the
// loop when KETE_ANTHROPIC_URL points at the local proxy.
func TestBypassHeader_SkipsCaptureAndInject(t *testing.T) {
	dir := t.TempDir()
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	t.Setenv("KETE_HOME", dir)
	t.Setenv("KETE_PROJECT", resolvedDir)
	t.Setenv("KETE_INJECT_MEMORY", "1")
	t.Setenv("KETE_DRIFT_ENABLED", "1")
	t.Setenv("KETE_CAPTURE_MIN_BYTES", "0")

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Seed a prior task — would normally be injected.
	if err := db.CreateTask(context.Background(), &store.Task{
		ID: "prior", ProjectPath: resolvedDir, Source: "manual",
		Goal: "earlier work",
	}); err != nil {
		t.Fatal(err)
	}

	var upstreamHits atomic.Int32
	var lastUpBody atomic.Pointer[[]byte]
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		body, _ := io.ReadAll(r.Body)
		stored := append([]byte(nil), body...)
		lastUpBody.Store(&stored)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"r","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer upstream.Close()

	cfg := Config{
		Host: "127.0.0.1", Port: 0,
		MaxBodyBytes: defaultMaxBodyBytes, RequestTimeout: 5 * time.Second,
	}
	srv := NewServer(cfg, db)
	srv.adapters[UpstreamAnthropic] = &anthropic.Adapter{
		BaseURL: upstream.URL, HTTPClient: http.DefaultClient,
	}

	front := httptest.NewServer(srv.http.Handler)
	defer front.Close()

	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"extract something"}]}`)

	// 1. Bypass-tagged request: no capture, no injection.
	req, _ := http.NewRequest("POST", front.URL+"/v1/messages", bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")
	req.Header.Set(extract.BypassHeader, "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if upstreamHits.Load() != 1 {
		t.Errorf("upstreamHits=%d, want 1", upstreamHits.Load())
	}
	got := *lastUpBody.Load()
	if bytes.Contains(got, []byte("prior")) || bytes.Contains(got, []byte("kete:memory")) {
		t.Errorf("bypass request had memory spliced in:\n%s", got)
	}

	// Wait briefly to catch any errant async capture, then verify
	// only the seeded task exists.
	srv.capture.Wait()
	tasks, _ := db.ListTasks(context.Background(), resolvedDir)
	if len(tasks) != 1 {
		t.Errorf("got %d tasks, want 1 (seeded only; bypass should not capture)",
			len(tasks))
	}

	// 2. Plain request (no bypass): should capture + inject as usual.
	req2, _ := http.NewRequest("POST", front.URL+"/v1/messages", bytes.NewReader(body))
	req2.Header.Set("content-type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()

	if resp2.StatusCode != 200 {
		t.Fatalf("status=%d", resp2.StatusCode)
	}
	got2 := *lastUpBody.Load()
	if !bytes.Contains(got2, []byte("kete:memory")) {
		t.Errorf("plain request did NOT have memory injected:\n%s", got2)
	}
	srv.capture.Wait()
	tasks, _ = db.ListTasks(context.Background(), resolvedDir)
	// seeded + this turn's capture
	if len(tasks) != 2 {
		t.Errorf("got %d tasks after plain request, want 2", len(tasks))
	}

	// Sanity: dummy unused vars to keep lints quiet on smaller go versions.
	_ = fmt.Sprintf("hits=%d", upstreamHits.Load())
}
