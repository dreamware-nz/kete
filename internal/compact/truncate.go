package compact

import (
	"encoding/json"
	"fmt"
)

// TruncateLargeBody is the safety valve for sessions that grew past
// the upstream's per-call token cap before the regular usage-driven
// compaction could fire. Apply needs a Summary, and Compute needs to
// fit the conversation through Haiku's 200k input cap — neither holds
// once the conversation alone is >1M tokens (which is what poisoned
// the live Crush session this was written for).
//
// Strategy: keep system / tools / top-level config untouched. Replace
// `messages` with a synthetic "earlier turns omitted" marker followed
// by the last keepLast entries. This is a hard cut, not a summary —
// the agent loses old context but the session stays alive.
//
// Returns the rewritten body and a true flag if truncation happened.
// If parsing fails or the message count is already at-or-below
// keepLast, returns the original body and false.
func TruncateLargeBody(rawBody []byte, keepLast int) ([]byte, bool, error) {
	if keepLast < 1 {
		keepLast = 1
	}
	var probe map[string]any
	if err := json.Unmarshal(rawBody, &probe); err != nil {
		return rawBody, false, fmt.Errorf("compact.TruncateLargeBody: parse: %w", err)
	}
	msgsAny, _ := probe["messages"].([]any)
	if len(msgsAny) <= keepLast+1 {
		return rawBody, false, nil
	}
	dropped := len(msgsAny) - keepLast
	marker := map[string]any{
		"role": "user",
		"content": []any{
			map[string]any{
				"type": "text",
				"text": fmt.Sprintf(
					"<kete:truncation>Earlier %d turns omitted: session crossed kete's hard size cap before usage-driven compaction could fire. Recent turns retained below; long-term memory still queryable via kete_preview / kete_expand.</kete:truncation>",
					dropped,
				),
			},
		},
	}
	probe["messages"] = append([]any{marker}, msgsAny[len(msgsAny)-keepLast:]...)
	out, err := json.Marshal(probe)
	if err != nil {
		return rawBody, false, fmt.Errorf("compact.TruncateLargeBody: remarshal: %w", err)
	}
	return out, true, nil
}
