package mcp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed tools/preview.txt
var previewDescription string

//go:embed tools/expand.txt
var expandDescription string

// toolDef is the shape MCP returns for tools/list.
type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// toolContent is one block of tool output. MCP supports text/image/etc;
// we only ever return text.
type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

func toolText(s string) toolResult {
	return toolResult{Content: []toolContent{{Type: "text", Text: s}}}
}

func toolError(s string) toolResult {
	return toolResult{Content: []toolContent{{Type: "text", Text: s}}, IsError: true}
}

func toolJSON(v any) toolResult {
	b, err := json.Marshal(v)
	if err != nil {
		return toolError(fmt.Sprintf("marshal result: %s", err))
	}
	return toolText(string(b))
}

func (s *Server) handleToolsList() (any, *rpcError) {
	return map[string]any{
		"tools": []toolDef{
			{
				Name:        "kete_preview",
				Description: previewDescription,
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"context": map[string]any{
							"type":        "string",
							"description": "user's prompt or short summary",
						},
						"mode": map[string]any{
							"type":        "string",
							"enum":        []string{"project", "all"},
							"description": "scope of search; default project",
						},
					},
					"required": []string{"context"},
				},
			},
			{
				Name:        "kete_expand",
				Description: expandDescription,
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "8-char id from a recent kete_preview",
						},
					},
					"required": []string{"id"},
				},
			},
		},
	}, nil
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	p, e := parseParams[toolCallParams](raw)
	if e != nil {
		return nil, e
	}
	switch p.Name {
	case "kete_preview":
		return s.callPreview(ctx, p.Arguments)
	case "kete_expand":
		return s.callExpand(ctx, p.Arguments)
	default:
		return nil, &rpcError{Code: errMethodNotFound, Message: "unknown tool: " + p.Name}
	}
}
