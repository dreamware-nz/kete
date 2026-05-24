package proxy

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/dreamware-nz/kete/internal/inject"
	"github.com/dreamware-nz/kete/internal/store"
)

// TestExpandLoop_ResolvesAndContinues runs a non-streaming POST
// through the proxy. The fake upstream emits a kete_expand tool_use
// on the first call and a normal text response on the second; the
// proxy should resolve the expand against the local store, build a
// continue body, and forward the final response to the client.
func TestExpandLoop_ResolvesAndContinues(t *testing.T) {
	dir := t.TempDir()
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	t.Setenv("KETE_HOME", dir)
	t.Setenv("KETE_PROJECT", resolvedDir)

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	taskID := "expandable-task"
	if err := db.CreateTask(context.Background(), &store.Task{
		ID:          taskID,
		ProjectPath: resolvedDir,
		Source:      "manual",
		Goal:        "build login flow",
		Decisions:   []store.Decision{{Choice: "JWT", Rationale: "stateless"}},
	}); err != nil {
		t.Fatal(err)
	}
	shortID := inject.ShortID(taskID)

	var hits atomic.Int32
	var lastBody atomic.Pointer[[]byte]
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		stored := append([]byte(nil), body...)
		lastBody.Store(&stored)
		n := hits.Add(1)
		w.Header().Set("content-type", "application/json")
		switch n {
		case 1:
			// First call: emit a tool_use for kete_expand referencing
			// our seeded task.
			fmt.Fprintf(w, `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"Let me check prior context."},{"type":"tool_use","id":"toolu_01","name":"kete_expand","input":{"id":%q}}],"stop_reason":"tool_use"}`, shortID)
		default:
			// Second call: the model has the tool_result; emit a
			// regular text completion.
			fmt.Fprintf(w, `{"id":"msg_2","type":"message","role":"assistant","content":[{"type":"text","text":"Got it: build login with JWT."}],"stop_reason":"end_turn"}`)
		}
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

	body := []byte(`{"model":"claude","messages":[{"role":"user","content":"do the work"}]}`)
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Fatalf("status=%d body=%s", resp.StatusCode, respBody)
	}
	if hits.Load() != 2 {
		t.Errorf("upstream hits=%d, want 2 (initial + continue)", hits.Load())
	}
	if !strings.Contains(string(respBody), "build login with JWT") {
		t.Errorf("client did not receive the FINAL message; got: %s", respBody)
	}
	// The 2nd upstream call's body should contain the tool_result and
	// the original user message (= conversation continued).
	final := *lastBody.Load()
	var probe struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(final, &probe); err != nil {
		t.Fatalf("parse continue body: %v", err)
	}
	if len(probe.Messages) < 3 {
		t.Fatalf("continue body has %d messages, want >= 3 (original user + assistant + tool_result)", len(probe.Messages))
	}
	// Memory injection may have added an extra user message between
	// the original and the loop's appended pair. Assert only that an
	// assistant message and a tool_result-bearing user message exist
	// in order — that's what continues the conversation correctly.
	var sawAssistant, sawToolResultAfter bool
	for _, m := range probe.Messages {
		if m["role"] == "assistant" {
			sawAssistant = true
			continue
		}
		if sawAssistant && m["role"] == "user" {
			b, _ := json.Marshal(m["content"])
			if bytes.Contains(b, []byte("tool_result")) {
				sawToolResultAfter = true
			}
		}
	}
	if !sawAssistant {
		t.Errorf("continue body missing assistant turn")
	}
	if !sawToolResultAfter {
		t.Errorf("continue body missing tool_result after assistant turn")
	}
	if !bytes.Contains(final, []byte("tool_result")) {
		t.Errorf("continue body missing tool_result block")
	}
	if !bytes.Contains(final, []byte("build login flow")) {
		t.Errorf("continue body missing the resolved goal — store→expand resolution failed")
	}
}

// TestExpandLoop_StreamingPassesThrough confirms streaming requests
// skip the orchestrator (Crush handles tool_use client-side via MCP).
func TestExpandLoop_StreamingPassesThrough(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("content-type", "text/event-stream")
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
		// Even if we emitted a tool_use here, the proxy must not run
		// the loop because stream:true.
		fmt.Fprint(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\",\"name\":\"kete_expand\"}}\n\n")
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
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

	body := []byte(`{"model":"claude","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if hits.Load() != 1 {
		t.Errorf("upstream hits=%d on streaming request, want 1 (loop skipped)", hits.Load())
	}
}

// TestExpandLoop_HitsCap stops at 5 cycles even if the model keeps
// emitting tool_use blocks.
func TestExpandLoop_HitsCap(t *testing.T) {
	dir := t.TempDir()
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	t.Setenv("KETE_HOME", dir)
	t.Setenv("KETE_PROJECT", resolvedDir)
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	taskID := "loopy"
	_ = db.CreateTask(context.Background(), &store.Task{
		ID: taskID, ProjectPath: resolvedDir, Source: "manual", Goal: "x",
	})
	shortID := inject.ShortID(taskID)

	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		// Always emit a tool_use; the cap is what stops the loop.
		fmt.Fprintf(w, `{"id":"m","content":[{"type":"tool_use","id":"toolu_x","name":"kete_expand","input":{"id":%q}}],"stop_reason":"tool_use"}`, shortID)
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

	body := []byte(`{"model":"claude","messages":[{"role":"user","content":"loop"}]}`)
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// 1 initial call + 5 continue calls = 6.
	const want = 1 + maxExpandCycles
	if got := int(hits.Load()); got != want {
		t.Errorf("upstream hits=%d, want %d (initial + %d cycles)", got, want, maxExpandCycles)
	}
}
