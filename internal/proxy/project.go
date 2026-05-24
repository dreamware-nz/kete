package proxy

import (
	"net/http"
	"os"
	"path/filepath"
)

// ProjectHeader is the per-request signal Crush (or any other client)
// sends to identify which project the prompt belongs to. Required when
// kete runs as a long-lived daemon that serves multiple projects.
const ProjectHeader = "X-Kete-Project"

// projectPath resolves the project identifier for an inbound request.
//
// Resolution order:
//  1. X-Kete-Project header — per-request, set by the client.
//  2. KETE_PROJECT env on the daemon — per-daemon, set when the proxy
//     is launched scoped to one project.
//  3. (deliberately) nothing else.
//
// We do NOT fall back to the daemon's cwd. Under launchd, kete's cwd
// is $HOME, which would (and did) bucket every project on the machine
// into one identity — making memory cross-pollute and drift fire on
// the wrong goal. Empty return value means "no project"; the caller
// must skip all project-keyed work (capture, inject, drift) for that
// request.
//
// Symlinks are resolved so /tmp aliases collapse to one identity.
func projectPath(h http.Header) string {
	if v := h.Get(ProjectHeader); v != "" {
		return resolve(v)
	}
	if v := os.Getenv("KETE_PROJECT"); v != "" {
		return resolve(v)
	}
	return ""
}

func resolve(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}
