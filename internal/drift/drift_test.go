package drift

import "testing"

func TestLevelFromScore(t *testing.T) {
	cases := []struct {
		score int
		want  Level
	}{
		{10, LevelNone}, {9, LevelNone}, {8, LevelNone},
		{7, LevelNudge},
		{6, LevelCorrect}, {5, LevelCorrect},
		{4, LevelIntervene}, {3, LevelIntervene},
		{2, LevelHalt}, {1, LevelHalt}, {0, LevelHalt},
		{-1, LevelHalt}, {99, LevelNone},
	}
	for _, c := range cases {
		if got := LevelFromScore(c.score); got != c.want {
			t.Errorf("score=%d got=%s want=%s", c.score, got, c.want)
		}
	}
}

func TestState_EscalationAndForcedRecovery(t *testing.T) {
	s := NewState()
	id := "session-1"
	if s.ForcedRecovery(id) {
		t.Error("forced too early")
	}
	s.Record(id, LevelCorrect)
	s.Record(id, LevelCorrect)
	if got := s.Escalation(id); got != 2 {
		t.Errorf("escalation=%d, want 2", got)
	}
	s.Record(id, LevelNone) // recovery
	if got := s.Escalation(id); got != 1 {
		t.Errorf("escalation=%d, want 1", got)
	}
	s.Record(id, LevelNudge)
	s.Record(id, LevelIntervene)
	s.Record(id, LevelHalt) // 3 corrections after the recovery
	if got := s.Escalation(id); got != 4 {
		t.Errorf("escalation=%d, want 4", got)
	}
	if !s.ForcedRecovery(id) {
		t.Error("expected forced recovery at escalation 4")
	}
}

func TestState_NeverNegative(t *testing.T) {
	s := NewState()
	id := "session-1"
	for i := 0; i < 5; i++ {
		s.Record(id, LevelNone)
	}
	if got := s.Escalation(id); got != 0 {
		t.Errorf("escalation=%d, want 0 (never negative)", got)
	}
}
