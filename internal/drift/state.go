package drift

import "sync"

// State is the per-session drift state: escalation counter and the
// last few scores. Lifetime: kete process. A new kete proxy run
// starts every session at 0.
//
// Forced-recovery mode (escalation >= 3) tells the orchestrator to
// inject a hard correction on the next prompt regardless of score.
type State struct {
	mu          sync.Mutex
	escalations map[string]int // session id -> count
}

const forcedRecoveryThreshold = 3

func NewState() *State {
	return &State{escalations: make(map[string]int)}
}

// Record updates the counter for sessionID after one drift check.
// Corrections (level > LevelNone) increment; recoveries (LevelNone)
// decrement; the counter never goes below zero.
func (s *State) Record(sessionID string, level Level) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.escalations[sessionID]
	if level == LevelNone {
		if cur > 0 {
			cur--
		}
	} else {
		cur++
	}
	s.escalations[sessionID] = cur
}

// Escalation returns the current count.
func (s *State) Escalation(sessionID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.escalations[sessionID]
}

// ForcedRecovery returns true when the escalation count for sessionID
// has crossed the threshold.
func (s *State) ForcedRecovery(sessionID string) bool {
	return s.Escalation(sessionID) >= forcedRecoveryThreshold
}

// Reset clears state for sessionID. Called when the user explicitly
// resets via the CLI; not called automatically.
func (s *State) Reset(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.escalations, sessionID)
}
