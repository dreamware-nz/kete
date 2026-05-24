package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/dreamware-nz/kete/internal/inject"
	"github.com/dreamware-nz/kete/internal/store"
)

// TestExpand_CrossProcess_FallsBackToStore proves the wire claim
// from plan 010 phase 6: a kete_expand call using only an 8-char
// short id resolves correctly even when the MCP cache has never
// seen the id (i.e. the proxy injected it from a different
// process). The fallback is store.FindByShortID + inject.ShortID.
func TestExpand_CrossProcess_FallsBackToStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Simulate "the proxy created this task in another process".
	taskID := "task-cross-process-123"
	if err := db.CreateTask(context.Background(), &store.Task{
		ID: taskID, ProjectPath: "/p", Source: "proxy",
		Goal:      "remember kowhai",
		Decisions: []store.Decision{{Choice: "X", Rationale: "Y"}},
	}); err != nil {
		t.Fatal(err)
	}

	// Compute the short id the same way the proxy would, the way the
	// model would see it in an injected memory.
	shortID := inject.ShortID(taskID)
	if len(shortID) != 8 {
		t.Fatalf("ShortID returned %q, want 8 chars", shortID)
	}

	// Drive an MCP server that has NEVER seen this id (no kete_preview
	// call). It should resolve via the store fallback.
	srv := NewServer(db, "test", io.Discard)
	pr, pw := io.Pipe()
	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(context.Background(), pr, &out)
	}()

	enc := json.NewEncoder(pw)
	_ = enc.Encode(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name":      "kete_expand",
			"arguments": map[string]any{"id": shortID},
		},
	})
	pw.Close()
	<-done

	sc := bufio.NewScanner(&out)
	if !sc.Scan() {
		t.Fatal("no response")
	}
	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\n%s", err, sc.Text())
	}
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
	if resp.Result.IsError {
		t.Fatalf("tool reported error: %+v", resp.Result.Content)
	}
	if len(resp.Result.Content) == 0 {
		t.Fatal("no content")
	}
	body := resp.Result.Content[0].Text
	if !strings.Contains(body, "remember kowhai") {
		t.Errorf("expand body missing goal: %s", body)
	}
	if !strings.Contains(body, `"choice":"X"`) {
		t.Errorf("expand body missing decision: %s", body)
	}
}

func TestExpand_CrossProcess_UnknownIDStillErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	srv := NewServer(db, "test", io.Discard)
	pr, pw := io.Pipe()
	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(context.Background(), pr, &out)
	}()
	enc := json.NewEncoder(pw)
	_ = enc.Encode(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name":      "kete_expand",
			"arguments": map[string]any{"id": "00000000"},
		},
	})
	pw.Close()
	<-done

	if !strings.Contains(out.String(), "unknown id") {
		t.Errorf("expected 'unknown id' tool-level error, got: %s", out.String())
	}
}
