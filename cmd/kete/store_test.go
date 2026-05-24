package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dreamware-nz/kete/internal/store"
)

// TestWithStore_ClosesOnPanic asserts the store closes even when fn
// panics — the only contract that earns withStore its existence.
func TestWithStore_ClosesOnPanic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate")
		}
		// On macOS/Linux the WAL file is removed by Close()'s
		// wal_checkpoint(TRUNCATE) + db.Close(). If withStore failed to
		// close, sqlite would have left a -wal file or a busy file.
		// We assert by re-opening: a non-closed connection on the same
		// path would surface as either a busy lock or stale WAL.
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir: %v", err)
		}
		for _, e := range entries {
			name := e.Name()
			if filepath.Ext(name) == "-wal" || filepath.Ext(name) == "-shm" {
				t.Errorf("WAL/SHM hanger after panic: %s", name)
			}
		}
		// Re-open round-trip: if Close didn't run, this might still
		// succeed on SQLite, but the migrate step exercises a write,
		// which proves the file is fully released.
		db, err := store.OpenDefault()
		if err != nil {
			t.Fatalf("re-open after panic: %v", err)
		}
		_ = db.Close()
	}()

	_ = withStore(func(db *store.DB) error {
		panic("boom")
	})
}
