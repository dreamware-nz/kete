package mcp

import (
	"context"
	"encoding/json"
)

type expandArgs struct {
	ID string `json:"id"`
}

type expandResult struct {
	Goal           string             `json:"goal"`
	Decisions      []store_Decision   `json:"decisions"`
	FilesTouched   []string           `json:"files_touched"`
	ReasoningTrace string             `json:"reasoning_trace"`
	CreatedAt      string             `json:"created_at"`
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
	realID, ok := s.cache.resolve(args.ID)
	if !ok {
		return toolError("unknown id; call kete_preview first this session"), nil
	}
	t, err := s.store.GetTask(ctx, realID)
	if err != nil {
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
