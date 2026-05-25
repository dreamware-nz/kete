package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Decision is the wire shape extraction returns. Mirrors store.Decision
// (which the capture pipeline copies into) but kept separate so the
// store package doesn't depend on extract.
type Decision struct {
	Choice    string `json:"choice"`
	Rationale string `json:"rationale"`
}

// TaskExtraction is the result of the goal/decisions/files extractor.
type TaskExtraction struct {
	Goal         string     `json:"goal"`
	Decisions    []Decision `json:"decisions"`
	FilesTouched []string   `json:"files_touched"`
}

// ExtractTask runs prompts/extract_task.txt against the conversation
// and returns the parsed extraction. Errors come from the network
// (already retried) or from a model response that isn't JSON.
//
// Uses the Anthropic assistant-message-prefill technique: we send a
// trailing assistant turn with content "{" so the model is forced to
// continue with JSON instead of starting a prose preamble. We then
// re-add the leading "{" before parsing. This is the standard fix
// for "ignore my JSON-only instruction" failures.
func (c *Client) ExtractTask(ctx context.Context, conversation string) (*TaskExtraction, error) {
	resp, err := c.SendWithRetry(ctx, Request{
		MaxTokens: MaxTokensExtractTask,
		System:    promptExtractTask,
		Messages: []Message{
			{Role: "user", Content: conversation},
			{Role: "assistant", Content: "{"},
		},
	})
	if err != nil {
		return nil, err
	}
	raw := "{" + resp.Text()
	var out TaskExtraction
	if err := json.Unmarshal([]byte(extractFromPrefilled(raw)), &out); err != nil {
		return nil, fmt.Errorf("ExtractTask: model returned non-JSON: %w (text: %s)", err, raw)
	}
	return &out, nil
}

// extractFromPrefilled trims the prefilled "{" assistant continuation
// to the outermost balanced { ... }, matching ExtractJSON's behaviour
// without the markdown-fence handling that doesn't apply here.
func extractFromPrefilled(s string) string {
	if i := strings.IndexByte(s, '{'); i >= 0 {
		if j := matchingBrace(s, i); j > 0 {
			return s[i : j+1]
		}
	}
	return s
}
