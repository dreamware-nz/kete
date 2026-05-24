package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dreamware-nz/kete/internal/adapter"
	"github.com/dreamware-nz/kete/internal/inject"
)

// runExpandLoop forwards body to ad, inspects the response for any
// `kete_expand` tool_use blocks the model emitted, resolves them via
// the local store, builds a continue-request, and re-forwards. Caps
// at maxExpandCycles per request (ADR 0011 / brief 002 / plan 002
// phase 16). Returns the final response status, headers, and bytes.
//
// Used only for non-streaming requests. Streaming requests skip the
// loop and pass through; Crush's client-side MCP integration handles
// kete_expand dispatch in that case.
func (s *Server) runExpandLoop(
	ctx context.Context,
	ad adapter.Wire,
	headers http.Header,
	body []byte,
) (status int, respHeaders http.Header, respBody []byte, err error) {
	guard := newExpandLoopGuard()
	cur := body
	for {
		bw := &bufferingWriter{header: make(http.Header)}
		if err := ad.Forward(ctx, cur, headers, bw); err != nil {
			return 0, nil, nil, err
		}
		// Empty status means handler didn't WriteHeader; treat as 200.
		if bw.status == 0 {
			bw.status = 200
		}
		if bw.status >= 400 {
			return bw.status, bw.header, bw.body.Bytes(), nil
		}

		toolUse := findKeteExpandToolUse(bw.body.Bytes())
		if toolUse == nil {
			return bw.status, bw.header, bw.body.Bytes(), nil
		}

		if guardErr := guard.Allow(); guardErr != nil {
			// Cap hit — return whatever we have. The client sees the
			// last partial response with an unresolved tool_use; a
			// well-behaved client falls back to its own tool dispatch.
			return bw.status, bw.header, bw.body.Bytes(), nil
		}

		toolResult := s.resolveExpandForLoop(ctx, toolUse.Input.ID)
		next, err := buildContinueBody(cur, bw.body.Bytes(), toolUse, toolResult)
		if err != nil {
			// If we can't build the continue body, return what we
			// have rather than silently looping.
			return bw.status, bw.header, bw.body.Bytes(), nil
		}
		cur = next
	}
}

// resolveExpandForLoop returns a tool_result content string for the
// given short id. Failure yields a tool_result with isError=true so
// the model gets feedback rather than the call going silent.
func (s *Server) resolveExpandForLoop(ctx context.Context, displayID string) string {
	t, err := s.store.FindByShortID(ctx, inject.ShortID, displayID)
	if err != nil {
		b, _ := json.Marshal(map[string]any{
			"error": "unknown id; not found in recent tasks",
			"id":    displayID,
		})
		return string(b)
	}
	out := map[string]any{
		"goal":            t.Goal,
		"decisions":       t.Decisions,
		"files_touched":   t.FilesTouched,
		"reasoning_trace": t.ReasoningTrace,
		"created_at":      t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// keteExpandToolUse is the slice of an Anthropic tool_use content
// block we care about.
type keteExpandToolUse struct {
	ID    string // toolu_...
	Input struct {
		ID string `json:"id"`
	}
}

// findKeteExpandToolUse scans an Anthropic-shaped non-streaming
// response body for the first content block that is a tool_use with
// name=kete_expand. Returns nil if none.
func findKeteExpandToolUse(respBody []byte) *keteExpandToolUse {
	var resp struct {
		Content []struct {
			Type  string          `json:"type"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil
	}
	for _, blk := range resp.Content {
		if blk.Type != "tool_use" || blk.Name != "kete_expand" {
			continue
		}
		out := &keteExpandToolUse{ID: blk.ID}
		_ = json.Unmarshal(blk.Input, &out.Input)
		return out
	}
	return nil
}

// buildContinueBody appends an assistant message (the model's
// response so far) and a user message (tool_result) to the original
// body. Re-marshals — we are explicitly outside ADR 0006's byte-exact
// rule here because we are introducing new conversation turns.
func buildContinueBody(origBody, respBody []byte, toolUse *keteExpandToolUse, toolResult string) ([]byte, error) {
	var orig map[string]any
	if err := json.Unmarshal(origBody, &orig); err != nil {
		return nil, fmt.Errorf("parse orig: %w", err)
	}
	var resp struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse resp: %w", err)
	}

	messagesAny, _ := orig["messages"].([]any)
	messagesAny = append(messagesAny,
		map[string]any{
			"role":    "assistant",
			"content": json.RawMessage(resp.Content),
		},
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": toolUse.ID,
					"content":     toolResult,
				},
			},
		},
	)
	orig["messages"] = messagesAny
	return json.Marshal(orig)
}

// bufferingWriter captures everything a handler writes — headers,
// status, and body — so the orchestrator can inspect before
// forwarding (or running another cycle).
type bufferingWriter struct {
	status int
	header http.Header
	body   bytes.Buffer
}

func (b *bufferingWriter) Header() http.Header { return b.header }
func (b *bufferingWriter) WriteHeader(s int)   { b.status = s }
func (b *bufferingWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	return b.body.Write(p)
}

// streamingRequest reports whether body has `"stream": true`. We
// peek without re-marshalling, the same way SelectUpstream does.
func streamingRequest(body []byte) bool {
	var probe struct {
		Stream bool `json:"stream"`
	}
	_ = json.Unmarshal(body, &probe)
	return probe.Stream
}
