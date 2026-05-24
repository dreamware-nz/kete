// Package extract is kete's Haiku-backed reasoning extractor.
//
// Always called against the Anthropic-direct API regardless of the
// user's chosen upstream — the user's own ANTHROPIC_API_KEY pays for
// the extraction calls. Pinned to claude-haiku-4-5-20251001 by
// default; KETE_DRIFT_MODEL overrides (ADR 0009).
package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	defaultModel   = "claude-haiku-4-5-20251001"
	defaultBaseURL = "https://api.anthropic.com"
	apiVersion     = "2023-06-01"
)

// Client is a tiny Anthropic Messages API client scoped to extraction
// use cases (small structured JSON outputs).
type Client struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// NewClient resolves config from env. Returns an error rather than
// papering over a missing key — callers must decide whether extraction
// is required for their flow (capture is fine without; drift is not).
func NewClient() (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, errors.New("extract: ANTHROPIC_API_KEY not set")
	}
	model := defaultModel
	if v := os.Getenv("KETE_DRIFT_MODEL"); v != "" {
		model = v
	}
	url := defaultBaseURL
	if v := os.Getenv("KETE_ANTHROPIC_URL"); v != "" {
		url = v
	}
	return &Client{
		BaseURL:    url,
		APIKey:     key,
		Model:      model,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Message is one Anthropic-shaped chat message; we only ever send
// `user` for extraction. content stays a string (not a typed block
// list) because extraction inputs are plain prose.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is the minimum body we need: model, max_tokens, system,
// messages. We hold max_tokens here so call sites can express their
// own budget per limits.go.
type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

// Response is the slice we read: id, content blocks, stop_reason,
// usage. Other fields ignored.
type Response struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Text returns the concatenation of all text content blocks. Most
// extraction calls produce one block; we concatenate defensively.
func (r *Response) Text() string {
	var b bytes.Buffer
	for _, c := range r.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

// Send POSTs req to /v1/messages and parses the response. Caller owns
// retry policy (see retry.go); Send is the bare wire op.
func (c *Client) Send(ctx context.Context, req Request) (*Response, error) {
	if req.Model == "" {
		req.Model = c.Model
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build req: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("anthropic-version", apiVersion)
	httpReq.Header.Set("x-api-key", c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, &HTTPError{Status: resp.StatusCode, Body: string(respBody)}
	}
	var out Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &out, nil
}

// HTTPError lets the retry layer distinguish 4xx (give up) from 5xx
// (try again).
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("anthropic %d: %s", e.Status, e.Body)
}

// IsRetryable returns true for 5xx and 429.
func (e *HTTPError) IsRetryable() bool {
	return e.Status >= 500 || e.Status == 429
}
