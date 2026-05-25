package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dreamware-nz/kete/internal/store"
)

const previewLimit = 3

type previewArgs struct {
	Context string `json:"context"`
	Mode    string `json:"mode"`
}

type previewItem struct {
	ID           string   `json:"id"`
	Summary      string   `json:"summary"`
	FilesTouched []string `json:"files_touched,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

type previewResult struct {
	Previews []previewItem `json:"previews"`
}

func (s *Server) callPreview(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	args, e := parseParams[previewArgs](raw)
	if e != nil {
		return nil, e
	}
	if args.Context == "" {
		return toolError("context is required"), nil
	}
	mode := args.Mode
	if mode == "" {
		mode = "project"
	}

	tasks, err := s.searchForPreview(ctx, args.Context, mode)
	if err != nil {
		return errToRPC(err)
	}
	// Drop unenriched rows — they have no goal, no decisions, no
	// files, so the previewer would just see id+timestamp. Empty
	// previews waste tokens and confuse agents.
	enriched := tasks[:0]
	for _, t := range tasks {
		if t.Goal != "" || len(t.Decisions) > 0 || len(t.FilesTouched) > 0 {
			enriched = append(enriched, t)
		}
	}
	tasks = enriched
	if len(tasks) > previewLimit {
		tasks = tasks[:previewLimit]
	}
	out := previewResult{Previews: make([]previewItem, 0, len(tasks))}
	for _, t := range tasks {
		out.Previews = append(out.Previews, previewItem{
			ID:           s.cache.register(t),
			Summary:      summarise(t),
			FilesTouched: t.FilesTouched,
			CreatedAt:    t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return toolJSON(out), nil
}

// searchForPreview tries SearchTasks first; if the query is empty-ish
// it falls back to ListTasks for the project. mode=all skips the
// project filter entirely.
func (s *Server) searchForPreview(ctx context.Context, query, mode string) ([]*store.Task, error) {
	tasks, err := s.store.SearchTasks(ctx, query)
	if err != nil {
		return nil, err
	}
	if mode == "all" {
		return tasks, nil
	}
	project, err := projectPath()
	if err != nil {
		return nil, err
	}
	filtered := tasks[:0]
	for _, t := range tasks {
		if t.ProjectPath == project {
			filtered = append(filtered, t)
		}
	}
	return filtered, nil
}

// summarise builds a one-line summary from goal + first decision.
func summarise(t *store.Task) string {
	if t.Goal != "" {
		return clip(t.Goal, 200)
	}
	if len(t.Decisions) > 0 {
		return clip(t.Decisions[0].Choice, 200)
	}
	return clip(t.ReasoningTrace, 200)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// projectPath mirrors cmd/kete's resolution; duplicated rather than
// imported because internal/cli imports internal/mcp eventually.
func projectPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		return real, nil
	}
	return cwd, nil
}
