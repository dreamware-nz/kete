package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dreamware-nz/kete/internal/adapter/anthropic"
	"github.com/dreamware-nz/kete/internal/extract"
	"github.com/dreamware-nz/kete/internal/store"
)

// TestProxy_CompactionFires_NextRequestRewritten:
//
//  1. Set warn=10 and clear=20 token thresholds via env so the test
//     can trip them with small fake usage numbers.
//  2. Send request #1 — upstream returns SSE with usage that crosses
//     BOTH warn and clear thresholds in one go.
//  3. Wait for the background StartCompute to finish so the cache has
//     a Summary.
//  4. Send request #2 — its body should have been rewritten by
//     compact.Apply: messages array replaced with [<summary>, <next prompt>].
func TestProxy_CompactionFires_NextRequestRewritten(t *testing.T) {
	dir := t.TempDir()
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	t.Setenv("KETE_HOME", dir)
	t.Setenv("KETE_PROJECT", resolvedDir)
	t.Setenv("KETE_COMPACT_WARN_TOKENS", "10")
	t.Setenv("KETE_COMPACT_CLEAR_TOKENS", "20")

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Forwarding upstream: returns SSE that reports 50 input + 50
	// output tokens, well past both thresholds.
	var upstreamHits atomic.Int32
	var lastUpBody atomic.Pointer[[]byte]
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		stored := append([]byte(nil), body...)
		lastUpBody.Store(&stored)
		upstreamHits.Add(1)

		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(200)
		fl, _ := w.(http.Flusher)
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"x\",\"usage\":{\"input_tokens\":50,\"output_tokens\":1}}}\n\n")
		fl.Flush()
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n")
		fl.Flush()
		fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":50}}\n\n")
		fl.Flush()
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		fl.Flush()
	}))
	defer upstream.Close()

	// Stubbed Haiku that responds to compact_summary prompts with a
	// real Summary, and to ExtractTask prompts with a goal.
	extractUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var text string
		switch {
		case bytes.Contains(body, []byte("structured summary")):
			text = `{\"summary\":{\"goal\":\"build login\",\"decisions\":[{\"choice\":\"jwt\",\"rationale\":\"stateless\"}],\"constraints\":[\"no breaking change\"],\"current_state\":\"scaffold built\",\"open_questions\":[]}}`
		case bytes.Contains(body, []byte("score")) && bytes.Contains(body, []byte("0-10")):
			text = `{\"score\":9,\"reasoning\":\"on track\",\"scope_violations\":[]}`
		default:
			text = `{\"goal\":\"login\",\"decisions\":[],\"files_touched\":[]}`
		}
		fmt.Fprintf(w, `{"id":"x","type":"message","role":"assistant","content":[{"type":"text","text":"%s"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`, text)
	}))
	defer extractUpstream.Close()

	cfg := Config{
		Host: "127.0.0.1", Port: 0,
		MaxBodyBytes: defaultMaxBodyBytes, RequestTimeout: 10 * time.Second,
	}
	srv := NewServer(cfg, db)
	srv.adapters[UpstreamAnthropic] = &anthropic.Adapter{
		BaseURL: upstream.URL, HTTPClient: http.DefaultClient,
	}
	stub := &extract.Client{
		BaseURL: extractUpstream.URL, APIKey: "sk-test",
		Model: "test", HTTPClient: http.DefaultClient,
	}
	srv.capture.SetExtractor(stub)
	srv.extractor = stub

	front := httptest.NewServer(srv.http.Handler)
	defer front.Close()

	body1 := []byte(`{"model":"claude","messages":[{"role":"user","content":"start work"}]}`)
	resp1, err := http.Post(front.URL+"/v1/messages", "application/json", bytes.NewReader(body1))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp1.Body)
	resp1.Body.Close()

	// Wait for the background compaction StartCompute to finish.
	srv.compactor.cache.Wait(resolvedDir)

	// Sanity: compactor saw warn+clear and queued apply.
	srv.compactor.mu.Lock()
	cs := srv.compactor.sessions[resolvedDir]
	srv.compactor.mu.Unlock()
	if cs == nil {
		t.Fatal("no compact state for project")
	}
	if !cs.applyPending {
		t.Errorf("applyPending=false, want true after crossing clear")
	}
	if _, ok := srv.compactor.cache.Get(resolvedDir); !ok {
		t.Fatal("expected cached summary; StartCompute may have failed")
	}

	// Request #2 — its body should be rewritten to drop the prior
	// conversation and inject the summary.
	body2 := []byte(`{"model":"claude","messages":[{"role":"user","content":"continue"}]}`)
	resp2, err := http.Post(front.URL+"/v1/messages", "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()

	if upstreamHits.Load() != 2 {
		t.Fatalf("upstreamHits=%d, want 2", upstreamHits.Load())
	}

	got := *lastUpBody.Load()
	if !strings.Contains(string(got), "kete:compaction") {
		t.Errorf("request #2 body was NOT rewritten by compact.Apply:\n%s", got)
	}
	if !strings.Contains(string(got), "build login") {
		t.Errorf("rewritten body missing summary goal:\n%s", got)
	}
	if !strings.Contains(string(got), "continue") {
		t.Errorf("rewritten body missing next prompt:\n%s", got)
	}
	// Drain in-flight extractor work so the deferred httptest.Close()
	// doesn't race.
	srv.capture.Wait()
	srv.compactor.cache.Wait(resolvedDir)
}
