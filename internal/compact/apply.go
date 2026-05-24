package compact

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Apply rewrites rawBody so its `messages` array is replaced with a
// single user message containing the rendered summary, plus the
// caller's freshly-arriving user prompt appended.
//
// We deliberately re-marshal here. ADR 0006 prefers raw passthrough,
// but compaction is the load-bearing exception: we are intentionally
// dropping the prior conversation, which means the cache prefix is
// going to break anyway. Better to start a clean prefix from a
// well-formed body than to slice around the array boundaries.
//
// Returns the rewritten body. nextPrompt is the new user message text
// (typically pulled from the last message in the original messages
// array).
func Apply(rawBody []byte, summary *Summary, nextPrompt string) ([]byte, error) {
	if summary == nil {
		return rawBody, fmt.Errorf("compact.Apply: nil summary")
	}
	var probe map[string]any
	if err := json.Unmarshal(rawBody, &probe); err != nil {
		return rawBody, fmt.Errorf("compact.Apply: parse: %w", err)
	}
	probe["messages"] = []any{
		map[string]any{"role": "user", "content": renderSummary(summary)},
		map[string]any{"role": "user", "content": nextPrompt},
	}
	out, err := json.Marshal(probe)
	if err != nil {
		return rawBody, fmt.Errorf("compact.Apply: remarshal: %w", err)
	}
	return out, nil
}

func renderSummary(s *Summary) string {
	var b strings.Builder
	b.WriteString("<kete:compaction>\n")
	b.WriteString("Goal: ")
	b.WriteString(s.Goal)
	b.WriteString("\n")
	if len(s.Decisions) > 0 {
		b.WriteString("Decisions:\n")
		for _, d := range s.Decisions {
			fmt.Fprintf(&b, "  - %s — %s\n", d.Choice, d.Rationale)
		}
	}
	if len(s.Constraints) > 0 {
		b.WriteString("Constraints:\n")
		for _, c := range s.Constraints {
			fmt.Fprintf(&b, "  - %s\n", c)
		}
	}
	if s.CurrentState != "" {
		fmt.Fprintf(&b, "Current state: %s\n", s.CurrentState)
	}
	if len(s.OpenQuestions) > 0 {
		b.WriteString("Open questions:\n")
		for _, q := range s.OpenQuestions {
			fmt.Fprintf(&b, "  - %s\n", q)
		}
	}
	b.WriteString("</kete:compaction>")
	return b.String()
}

// LastUserPrompt extracts the text of the last user-role message in
// rawBody. Returns "" if nothing matches; caller can fall back.
func LastUserPrompt(rawBody []byte) string {
	var probe struct {
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rawBody, &probe); err != nil {
		return ""
	}
	for i := len(probe.Messages) - 1; i >= 0; i-- {
		m := probe.Messages[i]
		if m.Role != "user" {
			continue
		}
		switch v := m.Content.(type) {
		case string:
			return v
		case []any:
			// Anthropic content-blocks. Concatenate any text blocks.
			var texts []string
			for _, blk := range v {
				bm, ok := blk.(map[string]any)
				if !ok {
					continue
				}
				if t, _ := bm["type"].(string); t == "text" {
					if s, _ := bm["text"].(string); s != "" {
						texts = append(texts, s)
					}
				}
			}
			if len(texts) > 0 {
				return strings.Join(texts, "\n")
			}
		}
	}
	return ""
}
