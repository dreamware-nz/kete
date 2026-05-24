package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dreamware-nz/kete/internal/adapter/anthropic"
	"github.com/dreamware-nz/kete/internal/store"
)

// TestProxy_CaptureAndInject runs a full request through the server,
// asserts the upstream sees the byte-exact (possibly injected) body,
// and asserts a tasks row lands in the DB after the request.
func TestProxy_CaptureAndInject(t *testing.T) {
	dir := t.TempDir()
	// Resolve symlinks once so KETE_PROJECT and the seeded ProjectPath
	// match what the proxy's projectPath() will derive.
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KETE_HOME", dir)
	t.Setenv("KETE_PROJECT", resolvedDir)

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Seed one prior task so injection has something to splice.
	priorID := "prior-task-1"
	if err := db.CreateTask(context.Background(), &store.Task{
		ID:          priorID,
		ProjectPath: resolvedDir,
		Source:      "manual",
		Goal:        "earlier work",
		Decisions:   []store.Decision{{Choice: "x", Rationale: "y"}},
	}); err != nil {
		t.Fatal(err)
	}

	// Fake Anthropic upstream.
	upstreamHits := 0
	var upstreamSawBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		upstreamSawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer upstream.Close()

	// Build a Server pointed at our fake upstream.
	cfg := Config{
		Host: "127.0.0.1", Port: 0,
		MaxBodyBytes: defaultMaxBodyBytes, RequestTimeout: 5 * time.Second,
	}
	srv := NewServer(cfg, db)
	// Override the anthropic adapter to point at the fake upstream.
	srv.adapters[UpstreamAnthropic] = &anthropic.Adapter{
		BaseURL: upstream.URL, HTTPClient: http.DefaultClient,
	}

	// Run on httptest.Server using the chi handler.
	front := httptest.NewServer(srv.http.Handler)
	defer front.Close()

	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}]}`)
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Fatalf("status=%d body=%s", resp.StatusCode, respBody)
	}
	if upstreamHits != 1 {
		t.Errorf("upstreamHits=%d, want 1", upstreamHits)
	}
	// Upstream should have seen the injected body — i.e. our prior
	// task's id should appear in what the upstream received.
	if !bytes.Contains(upstreamSawBody, []byte(priorID)) {
		t.Errorf("upstream did not see injected memory:\n%s", upstreamSawBody)
	}

	// Wait briefly for the async capture to land.
	srv.capture.Wait()
	tasks, err := db.ListTasks(context.Background(), resolvedDir)
	if err != nil {
		t.Fatal(err)
	}
	// Two tasks: the prior we seeded, and the captured one.
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	// The captured row is the one with source="proxy".
	var captured *store.Task
	for _, t := range tasks {
		if t.Source == "proxy" {
			captured = t
			break
		}
	}
	if captured == nil {
		t.Fatalf("no proxy-captured task found among %d", len(tasks))
	}
	if !strings.Contains(captured.ReasoningTrace, "hello") {
		t.Errorf("captured trace missing original prompt: %q", captured.ReasoningTrace)
	}
	// Belt-and-braces: capture is pre-injection, so the prior id must
	// NOT appear in the captured trace.
	if strings.Contains(captured.ReasoningTrace, priorID) {
		t.Errorf("captured trace contained injected memory; expected pre-inject body")
	}
}
