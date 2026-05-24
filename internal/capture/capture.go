// Package capture documents kete's capture sources and provides
// shared helpers (project-path normalisation, dedupe via sync_tracker).
//
// Sources:
//   - "proxy": the HTTP proxy (internal/proxy/capture.go) — primary,
//     covers Crush sessions running through kete proxy.
//   - "jsonl-poll": polls Crush's session JSONL files for sessions
//     that bypassed the proxy (e.g. a user ran Crush in a directory
//     before they remembered to start kete proxy). Future work; not
//     wired in v1.
//   - "manual": kete tasks add ... entered by a developer via the CLI
//     when capture failed or to seed knowledge. Future work.
//
// Brief 006's original five-source plan (proxy + IDE-specific scanners
// for Cursor, Zed, Codex, Antigravity) has collapsed to "Crush via
// the proxy". The other clients are not first-class, and capture for
// them lands when there's a real user. See process/drift.md.
package capture

import (
	"os"
	"path/filepath"
)

// NormaliseProject is the single canonical resolver for project_path.
// Both proxy capture and memory injection call this so the keys match.
func NormaliseProject(raw string) string {
	if raw == "" {
		var err error
		raw, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	if real, err := filepath.EvalSymlinks(raw); err == nil {
		return real
	}
	return raw
}

// TraceCap bounds reasoning_trace at storage time and at every read
// path that fans the trace back to a model. The raw request body for
// a long Crush session can be multiple MB; storing or returning it
// whole is what made expand return a 3 MB blob that Crush re-sent and
// Bedrock rejected for "prompt too long". 32 KiB is enough for a
// human-readable tail of the conversation and well under Haiku's
// extractor budget. The tail is what carries the most signal — the
// turn that just ended.
const TraceCap = 32 * 1024

// TruncatedMarker is prepended to a clipped trace so readers can tell.
const TruncatedMarker = "[kete: trace head truncated; tail kept]\n"

// ClipTrace returns at most TraceCap bytes of body, keeping the tail.
// When clipping happens, TruncatedMarker is prepended.
func ClipTrace(body []byte) string {
	if len(body) <= TraceCap {
		return string(body)
	}
	tail := body[len(body)-TraceCap+len(TruncatedMarker):]
	return TruncatedMarker + string(tail)
}
