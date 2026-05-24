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
	"sync/atomic"
	"testing"
	"time"

	"github.com/dreamware-nz/kete/internal/adapter/anthropic"
	"github.com/dreamware-nz/kete/internal/extract"
	"github.com/dreamware-nz/kete/internal/store"
)

// TestProxy_DriftFiresAndCorrectsNextRequest sends 6 requests, with
// the goal-bearing prior task seeded so drift has something to score
// against. The 5th request (interval=5) triggers the drift check; the
// stubbed Haiku returns a low score; the 6th request must contain the
// kete:correction segment in the body the upstream sees.
func TestProxy_DriftFiresAndCorrectsNextRequest(t *testing.T) {
	dir := t.TempDir()
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	t.Setenv("KETE_HOME", dir)
	t.Setenv("KETE_PROJECT", resolvedDir)

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.CreateTask(context.Background(), &store.Task{
		ID:          "seed-1",
		ProjectPath: resolvedDir,
		Source:      "manual",
		Goal:        "implement login flow",
	}); err != nil {
		t.Fatal(err)
	}

	var (
		upstreamHits  atomic.Int32
		latestUpBody  atomic.Pointer[[]byte]
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		latestUpBody.Store(&body)
		upstreamHits.Add(1)
		_, _ = w.Write([]byte(`{"id":"x","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer upstream.Close()

	// Stubbed Haiku with two endpoints baked in:
	//  - drift_score: returns score=2 (LevelHalt)
	//  - drift_correct: returns a correction message
	// We dispatch by inspecting the system prompt the client sent.
	var extractCalls atomic.Int32
	extractUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractCalls.Add(1)
		body, _ := io.ReadAll(r.Body)
		text := ""
		switch {
		case bytes.Contains(body, []byte("score")) && bytes.Contains(body, []byte("0-10")):
			text = `{\"score\":2,\"reasoning\":\"way off goal\",\"scope_violations\":[]}`
		case bytes.Contains(body, []byte("correction")):
			text = `{\"correction\":\"Return to the login flow.\"}`
		default:
			// extraction during capture: return a parseable goal.
			text = `{\"goal\":\"login\",\"decisions\":[],\"files_touched\":[]}`
		}
		fmt.Fprintf(w, `{"id":"x","type":"message","role":"assistant","content":[{"type":"text","text":"%s"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`, text)
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
	stub := &extract.Client{
		BaseURL: extractUpstream.URL, APIKey: "sk-test",
		Model: "test", HTTPClient: http.DefaultClient,
	}
	srv.capture.SetExtractor(stub)
	srv.extractor = stub

	front := httptest.NewServer(srv.http.Handler)
	defer front.Close()

	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hi"}]}`)
	post := func() {
		resp, err := http.Post(front.URL+"/v1/messages", "application/json",
			bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// 5 requests: the 5th should trigger drift scoring (interval=5).
	for i := 0; i < 5; i++ {
		post()
	}
	// Wait for the drift goroutine to land its correction.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		srv.mu.Lock()
		_, has := srv.corrections[resolvedDir]
		srv.mu.Unlock()
		if has {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 6th request — should pick up the queued correction.
	post()

	// Inspect the body the upstream saw on request #6.
	got := *latestUpBody.Load()
	if !strings.Contains(string(got), "kete:correction") {
		t.Errorf("6th request did not carry correction:\n%s", got)
	}
	if !strings.Contains(string(got), "Return to the login flow") {
		t.Errorf("6th request missing correction text:\n%s", got)
	}

	// The escalation counter should have ticked up.
	if e := srv.driftSt.Escalation(resolvedDir); e == 0 {
		t.Errorf("escalation=%d, want > 0", e)
	}

	// drift_log should have a row from the < 5 score.
	srv.capture.Wait()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM drift_log WHERE level = ?`, "halt").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Errorf("expected drift_log row(s) for halt, got 0")
	}
}
