package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dreamware-nz/kete/internal/store"
)

// rpc helpers for the test client.
type req struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type resp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// runRoundTrip drives the server end-to-end through a pipe and returns
// the parsed responses in order.
func runRoundTrip(t *testing.T, db *store.DB, msgs []req) []resp {
	t.Helper()
	pr, pw := io.Pipe()
	var out bytes.Buffer
	srv := NewServer(db, "test", io.Discard)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(context.Background(), pr, &out)
	}()
	enc := json.NewEncoder(pw)
	for _, m := range msgs {
		m.JSONRPC = "2.0"
		if err := enc.Encode(m); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	pw.Close()
	<-done

	var responses []resp
	sc := bufio.NewScanner(&out)
	for sc.Scan() {
		var r resp
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("decode response %q: %v", sc.Text(), err)
		}
		responses = append(responses, r)
	}
	return responses
}

func TestServer_PingInitializeToolsCall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	// Place cwd inside dir so projectPath() == dir for project mode.
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	// Seed two tasks.
	cwd, _ := os.Getwd()
	for i, goal := range []string{"refactor auth flow", "fix flaky test"} {
		err := db.CreateTask(context.Background(), &store.Task{
			ID:          "task-" + string(rune('0'+i)),
			ProjectPath: cwd,
			Source:      "test",
			Goal:        goal,
			Decisions:   []store.Decision{{Choice: "X", Rationale: "Y"}},
			ReasoningTrace: "trace " + goal,
		})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	out := runRoundTrip(t, db, []req{
		{ID: 1, Method: "ping"},
		{ID: 2, Method: "initialize", Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "0"},
		}},
		{ID: 3, Method: "tools/list"},
		{ID: 4, Method: "tools/call", Params: map[string]any{
			"name":      "kete_preview",
			"arguments": map[string]any{"context": "auth", "mode": "project"},
		}},
	})

	if len(out) != 4 {
		t.Fatalf("got %d responses, want 4: %+v", len(out), out)
	}
	for i, r := range out {
		if r.Error != nil {
			t.Fatalf("response %d errored: %+v", i, r.Error)
		}
	}

	// initialize → ProtocolVersion echoes.
	var initR initializeResult
	if err := json.Unmarshal(out[1].Result, &initR); err != nil {
		t.Fatalf("init result: %v", err)
	}
	if initR.ProtocolVersion != protocolVersion {
		t.Errorf("protocolVersion=%q want %q", initR.ProtocolVersion, protocolVersion)
	}

	// tools/list → both tools.
	var tl struct {
		Tools []toolDef `json:"tools"`
	}
	if err := json.Unmarshal(out[2].Result, &tl); err != nil {
		t.Fatalf("tools/list result: %v", err)
	}
	if len(tl.Tools) != 2 {
		t.Errorf("got %d tools, want 2", len(tl.Tools))
	}
	names := []string{tl.Tools[0].Name, tl.Tools[1].Name}
	if !contains(names, "kete_preview") || !contains(names, "kete_expand") {
		t.Errorf("tool names %v missing one of kete_preview/kete_expand", names)
	}

	// tools/call kete_preview → text result containing one preview.
	var pr toolResult
	if err := json.Unmarshal(out[3].Result, &pr); err != nil {
		t.Fatalf("preview result: %v", err)
	}
	if pr.IsError {
		t.Fatalf("preview marked isError: %+v", pr)
	}
	if len(pr.Content) == 0 || pr.Content[0].Type != "text" {
		t.Fatalf("preview content malformed: %+v", pr)
	}
	if !strings.Contains(pr.Content[0].Text, "auth") {
		t.Errorf("preview did not surface auth task: %s", pr.Content[0].Text)
	}

	// Now exercise the cache → expand round-trip.
	var inner previewResult
	if err := json.Unmarshal([]byte(pr.Content[0].Text), &inner); err != nil {
		t.Fatalf("preview json: %v", err)
	}
	if len(inner.Previews) == 0 {
		t.Fatalf("no previews returned")
	}
	id := inner.Previews[0].ID

	out2 := runRoundTrip(t, db, []req{
		// fresh server → cache empty → unknown id error.
	})
	_ = out2 // just exercise empty-input path

	// Same server has the cache; call expand on the existing srv via a
	// fresh round trip would lose state. Instead drive the existing
	// server by reaching into it directly.
	// (This is the only place a test reaches in; round-trip otherwise.)
	expandOut := runRoundTripOnServer(t, []req{
		{ID: 1, Method: "tools/call", Params: map[string]any{
			"name":      "kete_expand",
			"arguments": map[string]any{"id": id},
		}},
	}, NewServer(db, "test", io.Discard).withCacheSeed(id, inner.Previews[0]))
	if len(expandOut) != 1 || expandOut[0].Error != nil {
		t.Fatalf("expand: %+v", expandOut)
	}
	var er toolResult
	if err := json.Unmarshal(expandOut[0].Result, &er); err != nil {
		t.Fatalf("expand decode: %v", err)
	}
	if er.IsError {
		t.Fatalf("expand isError: %+v", er)
	}
	if !strings.Contains(er.Content[0].Text, "auth") {
		t.Errorf("expand body missing goal: %s", er.Content[0].Text)
	}
}

// withCacheSeed pre-registers a display id → real id mapping for the
// test. It returns the server for chaining.
func (s *Server) withCacheSeed(displayID string, p previewItem) *Server {
	// Reverse-lookup the real id by re-deriving from the stored set.
	// Tests seed sequential ids; we know the prefix.
	for _, real := range []string{"task-0", "task-1"} {
		if shortID(real) == displayID {
			s.cache.byID[displayID] = real
			break
		}
	}
	return s
}

// runRoundTripOnServer is like runRoundTrip but uses a caller-supplied
// server (so cache state survives between calls in a test).
func runRoundTripOnServer(t *testing.T, msgs []req, srv *Server) []resp {
	t.Helper()
	pr, pw := io.Pipe()
	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(context.Background(), pr, &out)
	}()
	enc := json.NewEncoder(pw)
	for _, m := range msgs {
		m.JSONRPC = "2.0"
		_ = enc.Encode(m)
	}
	pw.Close()
	<-done
	var responses []resp
	sc := bufio.NewScanner(&out)
	for sc.Scan() {
		var r resp
		_ = json.Unmarshal(sc.Bytes(), &r)
		responses = append(responses, r)
	}
	return responses
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
