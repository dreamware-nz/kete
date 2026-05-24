package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// dirPerm and filePerm enforce ADR 0004's tight-by-default layout.
const (
	dirPerm  os.FileMode = 0o700
	filePerm os.FileMode = 0o600
)

// DefaultDir returns the kete dotdir, honouring KETE_HOME for tests.
// It does not create the directory; callers who need that should use
// EnsureDefaultDir.
func DefaultDir() (string, error) {
	if d := os.Getenv("KETE_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, ".kete"), nil
}

// DefaultDBPath returns the resolved DB path, honouring KETE_DB_PATH
// (full override) and KETE_HOME (dir override). The containing dir is
// created at 0700 if absent; an existing dir is not chmod'ed.
func DefaultDBPath() (string, error) {
	if p := os.Getenv("KETE_DB_PATH"); p != "" {
		if err := ensureDir(filepath.Dir(p)); err != nil {
			return "", err
		}
		return p, nil
	}
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	if err := ensureDir(dir); err != nil {
		return "", err
	}
	return filepath.Join(dir, "memory.db"), nil
}

func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return nil
}
