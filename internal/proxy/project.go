package proxy

import (
	"os"
	"path/filepath"
)

// projectPath resolves the project identifier for a request. KETE_PROJECT
// wins; otherwise we use the cwd of the kete process. Symlinks are
// resolved so /tmp aliases collapse to one identity.
//
// Crush sessions inherit the kete process's cwd (kete is started by
// the user from inside the project tree), so the cwd derivation lines
// up with what the user expects.
func projectPath() string {
	if v := os.Getenv("KETE_PROJECT"); v != "" {
		if real, err := filepath.EvalSymlinks(v); err == nil {
			return real
		}
		return v
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		return real
	}
	return cwd
}
