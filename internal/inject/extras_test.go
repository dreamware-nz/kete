package inject

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dreamware-nz/kete/internal/store"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRankLocal_NewestFirstAndLimit(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	for i := range 5 {
		err := db.CreateTask(ctx, &store.Task{
			ID:          "t-" + string(rune('a'+i)),
			ProjectPath: "/proj",
			Source:      "manual",
			Goal:        "goal-" + string(rune('a'+i)),
		})
		if err != nil {
			t.Fatal(err)
		}
		// Force monotonically-increasing created_at so ordering is
		// deterministic.
		time.Sleep(2 * time.Millisecond)
	}

	tasks, err := RankLocal(ctx, db, "/proj", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("len=%d, want 3", len(tasks))
	}
	// Newest is the highest-letter goal we just inserted (e).
	if tasks[0].Goal != "goal-e" {
		t.Errorf("newest=%q, want goal-e", tasks[0].Goal)
	}

	all, err := RankLocal(ctx, db, "/proj", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Errorf("n=0 should return all; got %d", len(all))
	}
}

func TestPreview_Format(t *testing.T) {
	t1 := &store.Task{
		ID:           "abc",
		Goal:         "do the thing",
		Decisions:    []store.Decision{{Choice: "X", Rationale: "Y"}},
		FilesTouched: []string{"a.go", "b.go"},
		CreatedAt:    time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC),
	}
	out := Preview(t1)
	checks := []string{
		`<kete:memory id="abc" created="2026-05-24">`,
		`<goal>do the thing</goal>`,
		`<decision choice="X" rationale="Y"/>`,
		`<files>a.go, b.go</files>`,
		`</kete:memory>`,
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPreview_EscapesXML(t *testing.T) {
	t1 := &store.Task{Goal: "a < b > c & d"}
	out := Preview(t1)
	if strings.Contains(out, "<goal>a < b > c & d") {
		t.Errorf("unescaped XML in goal: %s", out)
	}
	if !strings.Contains(out, "&lt;") || !strings.Contains(out, "&gt;") || !strings.Contains(out, "&amp;") {
		t.Errorf("expected entities: %s", out)
	}
}

func TestLogInjection(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	// FK to tasks: seed.
	if err := db.CreateTask(ctx, &store.Task{
		ID: "task-1", ProjectPath: "/p", Source: "test",
	}); err != nil {
		t.Fatal(err)
	}
	if err := LogInjection(ctx, db, "task-1", "/p", "req-1"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM injection_log WHERE task_id = ? AND request_id = ?`,
		"task-1", "req-1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows=%d, want 1", n)
	}
}

func TestLogInjection_NilDB(t *testing.T) {
	if err := LogInjection(context.Background(), nil, "x", "/p", "r"); err != nil {
		t.Errorf("nil db should be no-op, got %v", err)
	}
}
