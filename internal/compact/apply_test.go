package compact

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dreamware-nz/kete/internal/extract"
)

func TestApply_RewritesMessages(t *testing.T) {
	in := []byte(`{"model":"claude","max_tokens":100,"messages":[
		{"role":"user","content":"hi"},
		{"role":"assistant","content":"hi back"},
		{"role":"user","content":"what's next?"}
	]}`)
	s := &Summary{
		Goal: "build login",
		Decisions: []extract.Decision{
			{Choice: "JWT", Rationale: "stateless"},
		},
		Constraints:  []string{"no breaking change"},
		CurrentState: "scaffold built",
	}
	out, err := Apply(in, s, "now do logout")
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		MaxTokens int    `json:"max_tokens"`
		Model     string `json:"model"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.Messages) != 2 {
		t.Fatalf("messages=%d, want 2 (summary + new prompt)", len(probe.Messages))
	}
	if !strings.Contains(probe.Messages[0].Content, "build login") {
		t.Errorf("summary missing goal: %s", probe.Messages[0].Content)
	}
	if !strings.Contains(probe.Messages[0].Content, "JWT") {
		t.Errorf("summary missing decision: %s", probe.Messages[0].Content)
	}
	if probe.Messages[1].Content != "now do logout" {
		t.Errorf("next prompt=%q", probe.Messages[1].Content)
	}
	// model + max_tokens preserved.
	if probe.Model != "claude" || probe.MaxTokens != 100 {
		t.Errorf("model=%q max=%d", probe.Model, probe.MaxTokens)
	}
}

func TestApply_NilSummary(t *testing.T) {
	if _, err := Apply([]byte(`{}`), nil, ""); err == nil {
		t.Error("want error for nil summary")
	}
}

func TestLastUserPrompt_StringContent(t *testing.T) {
	in := []byte(`{"messages":[
		{"role":"user","content":"first"},
		{"role":"assistant","content":"reply"},
		{"role":"user","content":"latest"}
	]}`)
	if got := LastUserPrompt(in); got != "latest" {
		t.Errorf("got %q, want latest", got)
	}
}

func TestLastUserPrompt_BlockContent(t *testing.T) {
	in := []byte(`{"messages":[
		{"role":"user","content":[{"type":"text","text":"hi"},{"type":"text","text":"world"}]}
	]}`)
	if got := LastUserPrompt(in); got != "hi\nworld" {
		t.Errorf("got %q", got)
	}
}

func TestLastUserPrompt_NoUserMessage(t *testing.T) {
	in := []byte(`{"messages":[{"role":"assistant","content":"x"}]}`)
	if got := LastUserPrompt(in); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
