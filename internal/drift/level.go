// Package drift implements kete's anti-drift detection.
//
// Score 1-10 against a session's stated goal via Haiku, mapped to one
// of four correction levels (ADR 0011). Wires from the proxy hot path
// every KETE_DRIFT_CHECK_INTERVAL prompts.
package drift

// Level is one of the four correction tiers from ADR 0011.
type Level int

const (
	LevelNone      Level = iota // 8-10
	LevelNudge                  // 7
	LevelCorrect                // 5-6
	LevelIntervene              // 3-4
	LevelHalt                   // 1-2
)

// String returns the wire-shaped level name.
func (l Level) String() string {
	switch l {
	case LevelNone:
		return "none"
	case LevelNudge:
		return "nudge"
	case LevelCorrect:
		return "correct"
	case LevelIntervene:
		return "intervene"
	case LevelHalt:
		return "halt"
	}
	return "unknown"
}

// LevelFromScore maps a 1-10 score to a Level per the brief 007 table.
// Out-of-range scores clamp.
func LevelFromScore(score int) Level {
	switch {
	case score >= 8:
		return LevelNone
	case score == 7:
		return LevelNudge
	case score >= 5:
		return LevelCorrect
	case score >= 3:
		return LevelIntervene
	default:
		return LevelHalt
	}
}
