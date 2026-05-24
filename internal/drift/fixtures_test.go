package drift

import (
	"strings"
	"testing"
)

func TestLoadFixtures_Coverage(t *testing.T) {
	fx, err := LoadFixtures("../../testdata/drift/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(fx) < 20 {
		t.Fatalf("got %d fixtures, want >= 20 (brief 007 phase 1)", len(fx))
	}

	seenLevels := map[Level]int{}
	seenIDs := map[string]bool{}
	for _, f := range fx {
		// Every fixture has the four required strings.
		if f.ID == "" || f.Goal == "" || f.Action == "" || f.ExpectedLevel == "" {
			t.Errorf("fixture %+v missing required field(s)", f)
		}
		// Labels are valid level strings.
		switch f.ExpectedLevel {
		case "none", "nudge", "correct", "intervene", "halt":
			// ok
		default:
			t.Errorf("fixture %s: invalid expected_level %q", f.ID, f.ExpectedLevel)
		}
		// IDs are unique.
		if seenIDs[f.ID] {
			t.Errorf("duplicate fixture id %q", f.ID)
		}
		seenIDs[f.ID] = true
		// LevelOf round-trips.
		seenLevels[LevelOf(f)]++
	}
	// All four bands plus LevelNone covered.
	for _, l := range []Level{LevelNone, LevelNudge, LevelCorrect, LevelIntervene, LevelHalt} {
		if seenLevels[l] == 0 {
			t.Errorf("no fixture covers level %s", l)
		}
	}
}

func TestLoadFixtures_BadPath(t *testing.T) {
	_, err := LoadFixtures("/nope/no.json")
	if err == nil || !strings.Contains(err.Error(), "read") {
		t.Errorf("err=%v, want a read error", err)
	}
}
