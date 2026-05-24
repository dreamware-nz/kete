package proxy

import (
	"errors"
	"testing"
)

func TestDriftHook_FiresOnInterval(t *testing.T) {
	h := &driftHook{interval: 5, fired: make(chan struct{}, 8)}
	for i := 1; i <= 16; i++ {
		got := h.Tick()
		want := i%5 == 0
		if got != want {
			t.Errorf("tick %d: got=%t want=%t", i, got, want)
		}
	}
	// Channel should have one entry per crossed boundary: 5, 10, 15.
	got := 0
	for {
		select {
		case <-h.fired:
			got++
			continue
		default:
		}
		break
	}
	if got != 3 {
		t.Errorf("fired channel got %d events, want 3", got)
	}
}

func TestCompactHook_FiresOnce(t *testing.T) {
	h := &compactHook{warnTokens: 100, clearTokens: 200}
	h.Observe(50)
	if h.preComputeFired || h.applyFired {
		t.Error("fired too early")
	}
	h.Observe(120)
	if !h.preComputeFired || h.applyFired {
		t.Error("warn should fire, clear should not")
	}
	h.Observe(220)
	if !h.applyFired {
		t.Error("clear should fire")
	}
	// Calling Observe again with the same usage shouldn't unfire.
	h.Observe(50)
	if !h.preComputeFired || !h.applyFired {
		t.Error("hooks should stay fired")
	}
}

func TestExpandLoopGuard_FifthAllowed_SixthRejected(t *testing.T) {
	g := newExpandLoopGuard()
	for i := 1; i <= 5; i++ {
		if err := g.Allow(); err != nil {
			t.Errorf("cycle %d: unexpected err %v", i, err)
		}
	}
	if err := g.Allow(); !errors.Is(err, ErrExpandLoopExceeded) {
		t.Errorf("6th cycle: err=%v, want ErrExpandLoopExceeded", err)
	}
}
