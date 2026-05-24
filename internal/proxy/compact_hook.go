package proxy

import (
	"os"
	"strconv"
)

// compactHook fires PreCompute and Apply events when usage crosses
// thresholds. Plan 008 wires the consumer; for now we expose a tiny
// API the orchestrator (or tests) can drive.
//
// Thresholds default to 160k / 180k tokens (brief 002 / brief 008);
// override via KETE_COMPACT_WARN_TOKENS / KETE_COMPACT_CLEAR_TOKENS.
type compactHook struct {
	warnTokens  int
	clearTokens int

	preComputeFired bool
	applyFired      bool
}

const (
	defaultCompactWarnTokens  = 160_000
	defaultCompactClearTokens = 180_000
)

func newCompactHook() *compactHook {
	return &compactHook{
		warnTokens:  envInt("KETE_COMPACT_WARN_TOKENS", defaultCompactWarnTokens),
		clearTokens: envInt("KETE_COMPACT_CLEAR_TOKENS", defaultCompactClearTokens),
	}
}

// Observe records a token-usage report. If usage crosses warn for the
// first time, PreCompute fires; if it crosses clear, Apply fires.
// Once fired, neither re-fires until Reset is called (per session).
func (c *compactHook) Observe(usageTokens int) {
	if !c.preComputeFired && usageTokens >= c.warnTokens {
		c.preComputeFired = true
	}
	if !c.applyFired && usageTokens >= c.clearTokens {
		c.applyFired = true
	}
}

func (c *compactHook) Reset() {
	c.preComputeFired = false
	c.applyFired = false
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
