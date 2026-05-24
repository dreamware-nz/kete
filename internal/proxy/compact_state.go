package proxy

import (
	"context"
	"sync"

	"github.com/dreamware-nz/kete/internal/compact"
	"github.com/dreamware-nz/kete/internal/extract"
)

// compactState tracks one session's compaction lifecycle:
//
//   - hook: token-usage observer with warn/clear thresholds.
//   - cache: pre-computed Summary by session id.
//   - applyPending: clear has fired but the next request hasn't been
//     rewritten yet.
type compactState struct {
	hook         *compactHook
	cache        *compact.Cache
	applyPending bool
}

// compactSessions is the per-project map.
type compactSessions struct {
	mu       sync.Mutex
	sessions map[string]*compactState
	cache    *compact.Cache // shared across all projects in this proxy
}

func newCompactSessions() *compactSessions {
	return &compactSessions{
		sessions: make(map[string]*compactState),
		cache:    compact.NewCache(),
	}
}

func (cs *compactSessions) get(project string) *compactState {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	s, ok := cs.sessions[project]
	if !ok {
		s = &compactState{
			hook:  newCompactHook(),
			cache: cs.cache,
		}
		cs.sessions[project] = s
	}
	return s
}

// observe records usage for project. If warn fires, kicks off a
// background StartCompute against `conversation`. If clear fires,
// flips applyPending.
func (cs *compactSessions) observe(ex *extract.Client, project, conversation string, total int) {
	if ex == nil {
		return
	}
	s := cs.get(project)
	cs.mu.Lock()
	wasPre := s.hook.preComputeFired
	wasApply := s.hook.applyFired
	s.hook.Observe(total)
	preFired := !wasPre && s.hook.preComputeFired
	applyFired := !wasApply && s.hook.applyFired
	if applyFired {
		s.applyPending = true
	}
	cs.mu.Unlock()

	if preFired {
		// Use background context — this work outlives the request.
		// StartCompute internally has its own goroutine and timeout.
		s.cache.StartCompute(context.Background(), ex, project, conversation)
	}
}

// drainPending returns (and clears) the applyPending flag. The next
// request's body should be rewritten via compact.Apply when this
// returns true.
func (cs *compactSessions) drainPending(project string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	s, ok := cs.sessions[project]
	if !ok || !s.applyPending {
		return false
	}
	s.applyPending = false
	s.hook.Reset()
	return true
}
