package extract

import (
	"context"
	"encoding/json"
	"fmt"
)

// DecisionsExtraction is the result of the decisions-only extractor.
// Used when we already have a goal and just want richer rationale, or
// when running drift detection that needs the decision list separately.
type DecisionsExtraction struct {
	Decisions []Decision `json:"decisions"`
}

// ExtractDecisions runs prompts/extract_decisions.txt against the
// conversation and returns the parsed decision list.
func (c *Client) ExtractDecisions(ctx context.Context, conversation string) (*DecisionsExtraction, error) {
	resp, err := c.SendWithRetry(ctx, Request{
		MaxTokens: MaxTokensExtractDecisions,
		System:    promptExtractDecisions,
		Messages: []Message{
			{Role: "user", Content: conversation},
		},
	})
	if err != nil {
		return nil, err
	}
	var out DecisionsExtraction
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		return nil, fmt.Errorf("ExtractDecisions: model returned non-JSON: %w (text: %s)", err, resp.Text())
	}
	return &out, nil
}
