package proxy

import "errors"

// ErrExpandLoopExceeded is returned when a kete_expand tool loop has
// run more than maxExpandCycles times in a single request. The
// orchestrator (plan 003 calls kete_expand from inside the proxy when
// the model issues the tool) short-circuits and returns whatever the
// model has so far.
var ErrExpandLoopExceeded = errors.New("expand loop exceeded 5 cycles")

const maxExpandCycles = 5

// expandLoopGuard counts kete_expand invocations within one request.
// Construction is per-request; this is intentionally not thread-safe
// because kete_expand is sequential within a single tool-loop.
type expandLoopGuard struct {
	cycles int
}

func newExpandLoopGuard() *expandLoopGuard {
	return &expandLoopGuard{}
}

// Allow increments the counter and returns nil while we're within
// the cap, ErrExpandLoopExceeded once we've hit it.
func (g *expandLoopGuard) Allow() error {
	if g.cycles >= maxExpandCycles {
		return ErrExpandLoopExceeded
	}
	g.cycles++
	return nil
}
