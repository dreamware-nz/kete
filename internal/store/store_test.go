package store

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	db, err := OpenDefault()
	if err != nil {
		t.Fatalf("OpenDefault: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestDefaultDirRespectsKeteHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	got, err := DefaultDir()
	if err != nil || got != dir {
		t.Fatalf("DefaultDir = %q, %v; want %q", got, err, dir)
	}
}

func TestDefaultDBPathCreatesDirAt0700(t *testing.T) {
	root := t.TempDir()
	keteHome := filepath.Join(root, "nested", "kete")
	t.Setenv("KETE_HOME", keteHome)

	p, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	if want := filepath.Join(keteHome, "memory.db"); p != want {
		t.Fatalf("path = %q, want %q", p, want)
	}
	info, err := os.Stat(keteHome)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("perm = %v, want 0700", info.Mode().Perm())
	}
}

func TestOpenAppliesWALAndMigrates(t *testing.T) {
	db := openTest(t)
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("scan journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}

	for _, table := range []string{"tasks", "steps", "drift_log", "sync_tracker", "schema_migrations"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := openTest(t)
	// Re-run on the same connection.
	if err := Migrate(db.DB); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if n != 5 {
		t.Fatalf("schema_migrations rows = %d, want 5", n)
	}
}

func TestTaskCRUD(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	want := &Task{
		ID:          "task-1",
		ProjectPath: "/repo/a",
		UserID:      "alice",
		SystemName:  "host",
		Goal:        "implement X",
		Decisions: []Decision{
			{Choice: "use SQLite", Rationale: "single user, durable"},
		},
		FilesTouched:   []string{"a.go", "b.go"},
		ReasoningTrace: "decided to start with the store",
		Source:         "proxy",
	}
	if err := db.CreateTask(ctx, want); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := db.GetTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.ID != want.ID || got.Goal != want.Goal || !reflect.DeepEqual(got.Decisions, want.Decisions) ||
		!reflect.DeepEqual(got.FilesTouched, want.FilesTouched) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", got, want)
	}

	if err := db.UpdateTask(ctx, "task-1", "implement X better",
		[]Decision{{Choice: "use WAL", Rationale: "concurrent reads"}},
		[]string{"a.go", "b.go", "c.go"}, "extended trace"); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	got, err = db.GetTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetTask after update: %v", err)
	}
	if got.Goal != "implement X better" || len(got.FilesTouched) != 3 || got.Decisions[0].Choice != "use WAL" {
		t.Fatalf("update not applied: %+v", got)
	}

	if _, err := db.GetTask(ctx, "missing"); err != ErrNotFound {
		t.Fatalf("GetTask(missing) err = %v, want ErrNotFound", err)
	}
}

func TestListAndSearchTasks(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	mk := func(id, proj, goal string) {
		if err := db.CreateTask(ctx, &Task{
			ID: id, ProjectPath: proj, Goal: goal, Source: "test",
		}); err != nil {
			t.Fatalf("CreateTask %s: %v", id, err)
		}
	}
	mk("a1", "/repo/a", "fix login bug")
	mk("a2", "/repo/a", "add logout")
	mk("b1", "/repo/b", "rewrite login")

	list, err := db.ListTasks(ctx, "/repo/a")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListTasks = %d, want 2", len(list))
	}

	hits, err := db.SearchTasks(ctx, "login")
	if err != nil {
		t.Fatalf("SearchTasks: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("SearchTasks(login) = %d, want 2", len(hits))
	}
}

func TestCloseTruncatesWAL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	db, err := OpenDefault()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.CreateTask(context.Background(), &Task{
		ID: "t1", ProjectPath: "/x", Source: "test",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	for _, name := range []string{"memory.db-wal", "memory.db-shm"} {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && info.Size() > 0 {
			t.Fatalf("%s still exists with size %d after Close", name, info.Size())
		}
	}
}
