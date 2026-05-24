package proxy

import (
	"os"
	"strconv"
	"sync"
)

// driftHook fires every Nth request. Plan 007 wires the consumer; for
// now it's a counter+channel that feeds a no-op handler.
type driftHook struct {
	interval int
	mu       sync.Mutex
	count    int
	fired    chan struct{}
}

const defaultDriftCheckInterval = 5

func newDriftHook() *driftHook {
	n := defaultDriftCheckInterval
	if v := os.Getenv("KETE_DRIFT_CHECK_INTERVAL"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			n = parsed
		}
	}
	return &driftHook{interval: n, fired: make(chan struct{}, 16)}
}

// Tick records one request. It returns true when the count crosses a
// boundary (5, 10, 15 ...). The fired channel is also notified for
// any consumer that prefers an event-driven shape.
func (d *driftHook) Tick() bool {
	d.mu.Lock()
	d.count++
	hit := d.count%d.interval == 0
	d.mu.Unlock()
	if hit {
		select {
		case d.fired <- struct{}{}:
		default:
		}
	}
	return hit
}

// Count returns how many ticks have been recorded. Test-only.
func (d *driftHook) Count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.count
}
