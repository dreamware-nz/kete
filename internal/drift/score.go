package drift

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dreamware-nz/kete/internal/extract"
)

// Score is one drift evaluation. ScopeViolations is a (possibly empty)
// list of files/components the action touched that aren't in scope.
type Score struct {
	Score           int      `json:"score"`
	Reasoning       string   `json:"reasoning"`
	ScopeViolations []string `json:"scope_violations"`
}

// ScoreAction asks the extractor to score `action` against `goal`.
// Returns the parsed score plus the derived Level. Errors propagate;
// callers are expected to score "no-action" in the error path.
func ScoreAction(ctx context.Context, c *extract.Client, goal, action string) (*Score, Level, error) {
	conversation := fmt.Sprintf("GOAL: %s\n\nLATEST ACTION: %s", goal, action)
	resp, err := c.SendWithRetry(ctx, extract.Request{
		MaxTokens: extract.MaxTokensDriftScore,
		System:    extract.Prompts["drift_score"],
		Messages: []extract.Message{
			{Role: "user", Content: conversation},
		},
	})
	if err != nil {
		return nil, LevelNone, err
	}
	var s Score
	if err := json.Unmarshal([]byte(resp.Text()), &s); err != nil {
		return nil, LevelNone, fmt.Errorf("drift score: model returned non-JSON: %w (text: %s)", err, resp.Text())
	}
	return &s, LevelFromScore(s.Score), nil
}

// BuildCorrection asks the extractor to draft a correction message for
// the level. We only call this for levels >= LevelNudge.
func BuildCorrection(ctx context.Context, c *extract.Client, goal, action string, level Level) (string, error) {
	if level == LevelNone {
		return "", nil
	}
	conversation := fmt.Sprintf("GOAL: %s\n\nLATEST ACTION: %s\n\nLEVEL: %s", goal, action, level)
	resp, err := c.SendWithRetry(ctx, extract.Request{
		MaxTokens: extract.MaxTokensDriftCorrect,
		System:    extract.Prompts["drift_correct"],
		Messages: []extract.Message{
			{Role: "user", Content: conversation},
		},
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Correction string `json:"correction"`
	}
	if err := json.Unmarshal([]byte(resp.Text()), &out); err != nil {
		return "", fmt.Errorf("drift correct: model returned non-JSON: %w (text: %s)", err, resp.Text())
	}
	return out.Correction, nil
}
