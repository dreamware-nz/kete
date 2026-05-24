package proxy

import (
	"os"
	"strconv"
	"strings"
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

	// Hard size cap on the *inbound* request body. Below this, we
	// trust usage-driven compaction. At-or-above, the proxy drops
	// the middle of the conversation (compact.TruncateLargeBody)
	// before forwarding so the request fits the upstream's per-call
	// cap. 1 MiB ≈ 250k tokens — well under Bedrock's 1M cap with
	// headroom for system + tools.
	defaultHardTruncateBytes = 1 << 20
	defaultHardTruncateKeep  = 30
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

// envBool reads a boolean env flag. "1", "true", "yes", "on" → true
// (case-insensitive). Anything else, including unset, returns def.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}
