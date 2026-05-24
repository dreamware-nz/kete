// Package store is kete's only durable state.
//
// The store is a single SQLite database at ~/.kete/memory.db (overridable via
// KETE_HOME or KETE_DB_PATH). It owns the schema and exposes a small typed
// API: Open, Close, and the per-table CRUD in tasks.go etc.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB with a path so Close can do its own housekeeping
// (WAL truncate) without callers having to remember.
type DB struct {
	*sql.DB
	Path string
}

// Open opens (or creates) the kete database at path, applies pragmas,
// and runs all pending migrations. The directory must already exist;
// callers who want auto-creation should use OpenDefault.
func Open(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := applyPragmas(sqldb); err != nil {
		sqldb.Close()
		return nil, err
	}
	if err := Migrate(sqldb); err != nil {
		sqldb.Close()
		return nil, err
	}
	return &DB{DB: sqldb, Path: path}, nil
}

// OpenDefault resolves the default kete database path (creating ~/.kete
// at 0700 if needed) and opens it.
func OpenDefault() (*DB, error) {
	path, err := DefaultDBPath()
	if err != nil {
		return nil, err
	}
	return Open(path)
}

// Close runs a final WAL checkpoint so the on-disk artefact is just
// memory.db with no -wal/-shm hangers-on, then closes the connection.
func (d *DB) Close() error {
	if _, err := d.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		// Best-effort; surface but still close.
		_ = d.DB.Close()
		return fmt.Errorf("wal_checkpoint: %w", err)
	}
	return d.DB.Close()
}

func applyPragmas(db *sql.DB) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA foreign_keys=ON`,
		`PRAGMA wal_checkpoint(TRUNCATE)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("pragma %q: %w", s, err)
		}
	}
	return nil
}
