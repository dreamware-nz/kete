// Package compact is kete's auto-compaction layer.
//
// When a session's input+output token usage crosses warn (default
// 160_000), pre-compute a structured summary in the background. When
// it crosses clear (default 180_000), splice the summary into the
// next request as the first user message and drop the prior
// conversation. ADR-driven byte-exact discipline still applies.
package compact

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dreamware-nz/kete/internal/extract"
)

// Summary is the structured replacement for a long conversation.
type Summary struct {
	Goal           string             `json:"goal"`
	Decisions      []extract.Decision `json:"decisions"`
	Constraints    []string           `json:"constraints"`
	CurrentState   string             `json:"current_state"`
	OpenQuestions  []string           `json:"open_questions"`
}

// Compute runs the compact_summary.txt prompt against the captured
// conversation and returns the parsed Summary.
func Compute(ctx context.Context, c *extract.Client, conversation string) (*Summary, error) {
	resp, err := c.SendWithRetry(ctx, extract.Request{
		MaxTokens: extract.MaxTokensCompactSummary,
		System:    extract.Prompts["compact_summary"],
		Messages: []extract.Message{
			{Role: "user", Content: conversation},
		},
	})
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Summary Summary `json:"summary"`
	}
	if err := json.Unmarshal([]byte(resp.ExtractJSON()), &wrapper); err != nil {
		return nil, fmt.Errorf("compact summary: model returned non-JSON: %w (text: %s)", err, resp.Text())
	}
	return &wrapper.Summary, nil
}

// Cache holds per-session pre-computed summaries. Pre-compute fires at
// warn; Apply pulls from here at clear. If the pre-compute hasn't
// finished by the time Apply needs it, the orchestrator falls back to
// a synchronous Compute.
type Cache struct {
	mu        sync.Mutex
	summaries map[string]*Summary // session id -> summary
	pending   map[string]chan struct{} // session id -> done channel
}

func NewCache() *Cache {
	return &Cache{
		summaries: make(map[string]*Summary),
		pending:   make(map[string]chan struct{}),
	}
}

// StartCompute fires Compute in a goroutine and stores the result. If
// already computing or computed, this is a no-op.
func (c *Cache) StartCompute(ctx context.Context, client *extract.Client, sessionID, conversation string) {
	c.mu.Lock()
	if _, ok := c.summaries[sessionID]; ok {
		c.mu.Unlock()
		return
	}
	if _, ok := c.pending[sessionID]; ok {
		c.mu.Unlock()
		return
	}
	done := make(chan struct{})
	c.pending[sessionID] = done
	c.mu.Unlock()

	go func() {
		defer close(done)
		s, err := Compute(ctx, client, conversation)
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.pending, sessionID)
		if err == nil && s != nil {
			c.summaries[sessionID] = s
		}
	}()
}

// Get returns the cached summary if present. ok is false if no
// summary is computed (or pre-compute failed); caller falls back to
// synchronous Compute.
func (c *Cache) Get(sessionID string) (*Summary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.summaries[sessionID]
	return s, ok
}

// Wait blocks until any pre-compute for sessionID finishes (or
// returns immediately if none pending).
func (c *Cache) Wait(sessionID string) {
	c.mu.Lock()
	ch := c.pending[sessionID]
	c.mu.Unlock()
	if ch != nil {
		<-ch
	}
}
