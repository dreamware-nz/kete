// Package inject contains byte-offset edits and the shared
// preview-id derivation used by both the proxy (when splicing
// memories) and the MCP server (when serving kete_preview /
// kete_expand). Cross-process consistency is the load-bearing
// invariant — see ShortID below.
package inject

import (
	"crypto/sha1"
	"encoding/hex"
)

// ShortID returns the deterministic 8-char display id for a task.
// SHA1(task.ID), first 8 hex chars. Two cooperating processes
// (kete proxy, kete mcp) compute the same id for the same task,
// so an id mentioned in an injected memory by the proxy resolves
// in the MCP server without any shared map.
func ShortID(taskID string) string {
	sum := sha1.Sum([]byte(taskID))
	return hex.EncodeToString(sum[:])[:8]
}
