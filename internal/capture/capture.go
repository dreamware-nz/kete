// Package capture documents kete's capture sources and provides
// shared helpers (project-path normalisation, dedupe via sync_tracker).
//
// Sources:
//   - "proxy": the HTTP proxy (internal/proxy/capture.go) — primary,
//     covers Crush sessions running through kete proxy.
//   - "jsonl-poll": polls Crush's session JSONL files for sessions
//     that bypassed the proxy (e.g. a user ran Crush in a directory
//     before they remembered to start kete proxy). Future work; not
//     wired in v1.
//   - "manual": kete tasks add ... entered by a developer via the CLI
//     when capture failed or to seed knowledge. Future work.
//
// Brief 006's original five-source plan (proxy + IDE-specific scanners
// for Cursor, Zed, Codex, Antigravity) has collapsed to "Crush via
// the proxy". The other clients are not first-class, and capture for
// them lands when there's a real user. See process/drift.md.
package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormaliseProject is the single canonical resolver for project_path.
// Both proxy capture and memory injection call this so the keys match.
func NormaliseProject(raw string) string {
	if raw == "" {
		var err error
		raw, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	if real, err := filepath.EvalSymlinks(raw); err == nil {
		return real
	}
	return raw
}

// TraceCap bounds reasoning_trace at storage time and at every read
// path that fans the trace back to a model. 32 KiB is enough for a
// human-readable conversation and well under Haiku's extractor budget.
const TraceCap = 32 * 1024

// TruncatedMarker is prepended to a clipped trace so readers can tell.
const TruncatedMarker = "[kete: trace head truncated; tail kept]\n"

// ClipTrace returns at most TraceCap bytes of body, keeping the tail.
// When clipping happens, TruncatedMarker is prepended. Used as a
// fallback when the body isn't an Anthropic-shaped messages request.
func ClipTrace(body []byte) string {
	if len(body) <= TraceCap {
		return string(body)
	}
	tail := body[len(body)-TraceCap+len(TruncatedMarker):]
	return TruncatedMarker + string(tail)
}

// ExtractConversation pulls a human-readable conversation out of an
// Anthropic-shaped POST /v1/messages request body and returns at most
// TraceCap bytes of it. Unlike ClipTrace, this drops the `tools`
// array (which can be larger than the conversation in Crush sessions
// — the Slack MCP tool schema alone is several KiB), drops `system`
// preamble noise, and renders messages as `ROLE: text`.
//
// Falls back to ClipTrace(body) if the body doesn't parse as the
// expected shape, so non-conversation traffic still gets stored.
func ExtractConversation(body []byte) string {
	var probe struct {
		System   any `json:"system"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &probe); err != nil || probe.Messages == nil {
		return ClipTrace(body)
	}
	var b strings.Builder
	if sys := flattenSystem(probe.System); sys != "" {
		fmt.Fprintf(&b, "SYSTEM: %s\n\n", sys)
	}
	for _, m := range probe.Messages {
		fmt.Fprintf(&b, "%s: %s\n\n", strings.ToUpper(m.Role), flattenContent(m.Content))
	}
	out := strings.TrimRight(b.String(), "\n")
	// Apply the cap, keeping the tail (most recent turn).
	return ClipTrace([]byte(out))
}

// flattenContent renders a message's content field (string or list
// of typed blocks) as a single readable string. Tool definitions and
// large tool_result payloads are summarised, not inlined.
func flattenContent(raw json.RawMessage) string {
	// Try string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Fall through to typed blocks.
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return string(raw)
	}
	var parts []string
	for _, blk := range blocks {
		t, _ := blk["type"].(string)
		switch t {
		case "text":
			if v, _ := blk["text"].(string); v != "" {
				parts = append(parts, v)
			}
		case "tool_use":
			name, _ := blk["name"].(string)
			parts = append(parts, fmt.Sprintf("[tool_use: %s]", name))
		case "tool_result":
			if c, ok := blk["content"]; ok {
				parts = append(parts, fmt.Sprintf("[tool_result: %s]", clip(fmt.Sprint(c), 400)))
			} else {
				parts = append(parts, "[tool_result]")
			}
		default:
			parts = append(parts, "["+t+"]")
		}
	}
	return strings.Join(parts, "\n")
}

// flattenSystem accepts both a bare string and the typed-block form
// Anthropic accepts for system. Returns the concatenated text.
func flattenSystem(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []any:
		var parts []string
		for _, blk := range s {
			bm, ok := blk.(map[string]any)
			if !ok {
				continue
			}
			if text, _ := bm["text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
