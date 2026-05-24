package proxy

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dreamware-nz/kete/internal/store"
)

// TestBuildMemoryPayload_TypedBlockContent: Bedrock requires
// messages[*].content to be a list of typed blocks, not a bare
// string. Anthropic-direct accepts both, but injecting a
// string-content message into an otherwise block-content
// conversation produces "messages.N.content.0.type: Field required".
// This was a real live failure caught against Crush + Bedrock.
func TestBuildMemoryPayload_TypedBlockContent(t *testing.T) {
	tasks := []*store.Task{
		{ID: "t-1", Goal: "fix login", CreatedAt: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)},
	}
	out, err := buildMemoryPayload(tasks)
	if err != nil {
		t.Fatal(err)
	}
	var msg map[string]any
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatal(err)
	}
	if msg["role"] != "user" {
		t.Errorf("role=%v, want user", msg["role"])
	}
	c, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("content is not a list: %T (%v)", msg["content"], msg["content"])
	}
	if len(c) == 0 {
		t.Fatal("content list empty")
	}
	first, ok := c[0].(map[string]any)
	if !ok {
		t.Fatalf("first block is not an object: %T", c[0])
	}
	if first["type"] != "text" {
		t.Errorf(`first.type=%v, want "text"`, first["type"])
	}
	text, _ := first["text"].(string)
	if text == "" || !contains(text, "fix login") {
		t.Errorf("first.text missing goal: %q", text)
	}
}

func TestInjectCorrectionPayload_TypedBlockContent(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	out, err := injectCorrectionPayload(in, "stay on task")
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Messages []struct {
			Role    string           `json:"role"`
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatalf("not block-shaped JSON: %v\nout: %s", err, out)
	}
	if len(probe.Messages) != 2 {
		t.Fatalf("messages=%d, want 2", len(probe.Messages))
	}
	last := probe.Messages[1]
	if last.Role != "user" {
		t.Errorf("role=%q", last.Role)
	}
	if len(last.Content) == 0 || last.Content[0]["type"] != "text" {
		t.Errorf("correction content not typed-block: %+v", last.Content)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
