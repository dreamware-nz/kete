package mcp

import (
	"crypto/sha1"
	"encoding/hex"
	"sync"

	"github.com/dreamware-nz/kete/internal/store"
)

// previewCache maps an 8-char display id to a real task id. Lifetime
// is a single MCP server process — by the time the user starts a new
// kete mcp, fresh ids will be issued.
//
// The 8-char id is derived from sha1(task.id) so the same task gets
// the same display id across previews within a session, which makes
// the wire surface stable for the model.
type previewCache struct {
	mu    sync.RWMutex
	byID  map[string]string // 8-char display id -> store task id
}

func newPreviewCache() *previewCache {
	return &previewCache{byID: make(map[string]string)}
}

// shortID is the deterministic 8-char display id for a task.
func shortID(taskID string) string {
	sum := sha1.Sum([]byte(taskID))
	return hex.EncodeToString(sum[:])[:8]
}

// register records the display→real mapping and returns the display id.
func (c *previewCache) register(t *store.Task) string {
	id := shortID(t.ID)
	c.mu.Lock()
	c.byID[id] = t.ID
	c.mu.Unlock()
	return id
}

// resolve returns the real task id for a display id, or "" if unknown.
func (c *previewCache) resolve(displayID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	real, ok := c.byID[displayID]
	return real, ok
}
