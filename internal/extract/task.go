package extract

import (
	"context"
	"encoding/json"
	"fmt"
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
func (c *Client) ExtractTask(ctx context.Context, conversation string) (*TaskExtraction, error) {
	resp, err := c.SendWithRetry(ctx, Request{
		MaxTokens: MaxTokensExtractTask,
		System:    promptExtractTask,
		Messages: []Message{
			{Role: "user", Content: conversation},
		},
	})
	if err != nil {
		return nil, err
	}
	var out TaskExtraction
	if err := json.Unmarshal([]byte(resp.ExtractJSON()), &out); err != nil {
		return nil, fmt.Errorf("ExtractTask: model returned non-JSON: %w (text: %s)", err, resp.Text())
	}
	return &out, nil
}
