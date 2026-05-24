package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dreamware-nz/kete/internal/adapter/anthropic"
	"github.com/dreamware-nz/kete/internal/extract"
	"github.com/dreamware-nz/kete/internal/inject"
	"github.com/dreamware-nz/kete/internal/store"
)

// TestProxy_CaptureInjectAndEnrich runs a full request through the
// server, asserts:
//  1. the upstream sees the injected body (prior task id present)
//  2. the response streams back to the client
//  3. a proxy-source row lands in the DB with the pre-injection trace
//  4. the captured row is enriched (goal + decisions filled in by
//     ExtractTask, which we route through a fake Anthropic upstream)
func TestProxy_CaptureInjectAndEnrich(t *testing.T) {
	dir := t.TempDir()
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

	// Fake forwarding upstream: receives the proxied messages.
	upstreamHits := 0
	var upstreamSawBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		upstreamSawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer upstream.Close()

	// Fake Haiku: returns a fixed extraction so we can assert the
	// enrichment landed in the DB.
	extractHits := 0
	extractUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractHits++
		// Anthropic-shaped response with a single text block whose
		// content is the JSON ExtractTask expects.
		fmt.Fprint(w, `{"id":"x","type":"message","role":"assistant","content":[{"type":"text","text":"{\"goal\":\"answer hello\",\"decisions\":[{\"choice\":\"reply with ok\",\"rationale\":\"shortest valid response\"}],\"files_touched\":[\"none\"]}"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer extractUpstream.Close()

	cfg := Config{
		Host: "127.0.0.1", Port: 0,
		MaxBodyBytes: defaultMaxBodyBytes, RequestTimeout: 5 * time.Second,
	}
	srv := NewServer(cfg, db)
	srv.adapters[UpstreamAnthropic] = &anthropic.Adapter{
		BaseURL: upstream.URL, HTTPClient: http.DefaultClient,
	}
	// Inject a stubbed extractor pointed at our fake Haiku.
	srv.capture.SetExtractor(&extract.Client{
		BaseURL:    extractUpstream.URL,
		APIKey:     "sk-test",
		Model:      "test-haiku",
		HTTPClient: http.DefaultClient,
	})

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
	// The injected memory carries the 8-char ShortID (so MCP can
	// resolve it via kete_expand cross-process), not the full UUID.
	wantInjected := inject.ShortID(priorID)
	if !bytes.Contains(upstreamSawBody, []byte(wantInjected)) {
		t.Errorf("upstream did not see injected memory (looking for short id %q):\n%s",
			wantInjected, upstreamSawBody)
	}

	srv.capture.Wait()

	if extractHits != 1 {
		t.Errorf("extractHits=%d, want 1 (capture must enrich)", extractHits)
	}

	tasks, err := db.ListTasks(context.Background(), resolvedDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	var captured *store.Task
	for _, t := range tasks {
		if t.Source == "proxy" {
			captured = t
			break
		}
	}
	if captured == nil {
		t.Fatalf("no proxy-captured task found")
	}
	if !strings.Contains(captured.ReasoningTrace, "hello") {
		t.Errorf("trace missing original prompt: %q", captured.ReasoningTrace)
	}
	if strings.Contains(captured.ReasoningTrace, priorID) {
		t.Errorf("trace contained injected memory; expected pre-inject body")
	}
	// Enrichment landed.
	if captured.Goal != "answer hello" {
		t.Errorf("captured.Goal=%q, want 'answer hello' (enrichment failed)", captured.Goal)
	}
	if len(captured.Decisions) != 1 || captured.Decisions[0].Choice != "reply with ok" {
		t.Errorf("captured.Decisions=%+v, want one with choice 'reply with ok'", captured.Decisions)
	}
	if len(captured.FilesTouched) != 1 || captured.FilesTouched[0] != "none" {
		t.Errorf("captured.FilesTouched=%+v", captured.FilesTouched)
	}
}
