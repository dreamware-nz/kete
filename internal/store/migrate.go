package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate runs every migration in migrations/ that hasn't already been
// applied. Migrations are up-only (ADR 0003); failure aborts at the
// failing version and surfaces it.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := loadApplied(db)
	if err != nil {
		return err
	}

	files, err := listMigrations(migrationFS)
	if err != nil {
		return err
	}

	for _, m := range files {
		if applied[m.version] {
			continue
		}
		if err := applyOne(db, m); err != nil {
			return fmt.Errorf("migration %04d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadApplied(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func listMigrations(efs fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(efs, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	out := make([]migration, 0, len(names))
	for _, n := range names {
		v, err := parseVersion(n)
		if err != nil {
			return nil, err
		}
		body, err := fs.ReadFile(efs, "migrations/"+n)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", n, err)
		}
		out = append(out, migration{version: v, name: n, sql: string(body)})
	}
	return out, nil
}

func parseVersion(name string) (int, error) {
	// NNNN_description.sql
	idx := strings.IndexByte(name, '_')
	if idx <= 0 {
		return 0, fmt.Errorf("migration %q: missing NNNN_ prefix", name)
	}
	v, err := strconv.Atoi(name[:idx])
	if err != nil {
		return 0, fmt.Errorf("migration %q: bad version: %w", name, err)
	}
	return v, nil
}

func applyOne(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(m.sql); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`, m.version); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
