package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dreamware-nz/kete/internal/inject"
	"github.com/dreamware-nz/kete/internal/store"
)

type expandArgs struct {
	ID string `json:"id"`
}

type expandResult struct {
	Goal           string           `json:"goal"`
	Decisions      []store_Decision `json:"decisions"`
	FilesTouched   []string         `json:"files_touched"`
	ReasoningTrace string           `json:"reasoning_trace"`
	CreatedAt      string           `json:"created_at"`
}

// store_Decision mirrors store.Decision in the wire shape; aliased so
// preview/expand JSON stays decoupled from the store package's tag
// choices if they ever diverge.
type store_Decision struct {
	Choice    string `json:"choice"`
	Rationale string `json:"rationale"`
}

func (s *Server) callExpand(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	args, e := parseParams[expandArgs](raw)
	if e != nil {
		return nil, e
	}
	if args.ID == "" {
		return toolError("id is required"), nil
	}
	t, err := s.resolveTask(ctx, args.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return toolError("unknown id; not found in cache or recent tasks"), nil
		}
		return errToRPC(err)
	}
	out := expandResult{
		Goal:           t.Goal,
		FilesTouched:   t.FilesTouched,
		ReasoningTrace: t.ReasoningTrace,
		CreatedAt:      t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	for _, d := range t.Decisions {
		out.Decisions = append(out.Decisions, store_Decision(d))
	}
	return toolJSON(out), nil
}

// resolveTask returns the task whose 8-char display id matches
// displayID. Tries the in-memory cache first; falls back to a
// store-side scan keyed on inject.ShortID — that's how we resolve
// memories the proxy injected from a different process. (ADR 0008
// preview cache shape, plan 010 phase 6.)
func (s *Server) resolveTask(ctx context.Context, displayID string) (*store.Task, error) {
	if real, ok := s.cache.resolve(displayID); ok {
		return s.store.GetTask(ctx, real)
	}
	t, err := s.store.FindByShortID(ctx, inject.ShortID, displayID)
	if err != nil {
		return nil, err
	}
	// Record so a follow-up expand for the same id is O(1).
	s.cache.register(t)
	return t, nil
}
