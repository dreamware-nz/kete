package capture

import (
	"strings"
	"testing"
)

func TestExtractConversation_BasicShape(t *testing.T) {
	body := []byte(`{
		"model": "claude-x",
		"system": "you are kete",
		"tools": [{"name": "huge_tool", "description": "` + strings.Repeat("X", 5000) + `"}],
		"messages": [
			{"role": "user", "content": "find the bug in pagination"},
			{"role": "assistant", "content": "looking at /v1/users"}
		]
	}`)
	out := ExtractConversation(body)
	if !strings.Contains(out, "USER: find the bug in pagination") {
		t.Errorf("missing USER turn:\n%s", out)
	}
	if !strings.Contains(out, "ASSISTANT: looking at /v1/users") {
		t.Errorf("missing ASSISTANT turn:\n%s", out)
	}
	if !strings.Contains(out, "SYSTEM: you are kete") {
		t.Errorf("missing system:\n%s", out)
	}
	if strings.Contains(out, "huge_tool") {
		t.Errorf("tool definition leaked into trace")
	}
	if strings.Contains(out, strings.Repeat("X", 100)) {
		t.Errorf("tool description leaked into trace")
	}
}

func TestExtractConversation_TypedBlocks(t *testing.T) {
	body := []byte(`{
		"messages": [
			{"role": "user", "content": [
				{"type": "text", "text": "rebase on main"}
			]},
			{"role": "assistant", "content": [
				{"type": "text", "text": "ok"},
				{"type": "tool_use", "id": "x", "name": "Bash", "input": {}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "x", "content": "done"}
			]}
		]
	}`)
	out := ExtractConversation(body)
	if !strings.Contains(out, "rebase on main") {
		t.Errorf("text block missing:\n%s", out)
	}
	if !strings.Contains(out, "[tool_use: Bash]") {
		t.Errorf("tool_use marker missing:\n%s", out)
	}
	if !strings.Contains(out, "[tool_result:") {
		t.Errorf("tool_result marker missing:\n%s", out)
	}
}

func TestExtractConversation_FallsBackOnNonAnthropic(t *testing.T) {
	// Non-Anthropic body — falls through to ClipTrace which returns as-is.
	body := []byte(`{"foo": "bar"}`)
	out := ExtractConversation(body)
	if out != string(body) {
		t.Errorf("non-anthropic body not passed through: got=%q want=%q", out, body)
	}
}
