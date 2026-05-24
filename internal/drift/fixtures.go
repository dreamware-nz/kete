// Package drift includes a fixture loader and a runner used both
// from tests and from `kete drift-test --fixture <path>`.
package drift

import (
	"encoding/json"
	"fmt"
	"os"
)

// Fixture is one hand-labelled drift case: a goal, an action the
// agent took, and the level a human reviewer thinks it deserves.
//
// The score gap between adjacent levels is wide enough (3+) that a
// well-tuned model should reproduce these classifications almost
// every time. When it doesn't, it tells us the prompt or threshold
// table needs work.
type Fixture struct {
	ID            string `json:"id"`
	Goal          string `json:"goal"`
	Action        string `json:"action"`
	ExpectedLevel string `json:"expected_level"` // none|nudge|correct|intervene|halt
	Notes         string `json:"notes"`
}

// LoadFixtures reads fixtures from path. Errors propagate so the
// caller can fail fast.
func LoadFixtures(path string) ([]Fixture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("drift fixtures: read %s: %w", path, err)
	}
	var out []Fixture
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("drift fixtures: parse %s: %w", path, err)
	}
	return out, nil
}

// LevelOf returns the typed Level for the fixture's expected_level
// string. Unknown strings clamp to LevelNone — the loader test
// catches bad labels.
func LevelOf(f Fixture) Level {
	switch f.ExpectedLevel {
	case "none":
		return LevelNone
	case "nudge":
		return LevelNudge
	case "correct":
		return LevelCorrect
	case "intervene":
		return LevelIntervene
	case "halt":
		return LevelHalt
	}
	return LevelNone
}
