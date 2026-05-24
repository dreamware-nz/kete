package drift

import (
	"context"
	"testing"

	"github.com/dreamware-nz/kete/internal/store"
)

// TestPersist_StepsVsDriftLog: score >= 5 lands in steps; < 5 lands
// in drift_log with the correction text.
func TestPersist_StepsVsDriftLog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KETE_HOME", dir)
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Seed a parent task so steps/drift_log FKs resolve.
	taskID := "t-1"
	if err := db.CreateTask(context.Background(), &store.Task{
		ID: taskID, ProjectPath: "/p", Source: "test",
	}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Score 7 → step row.
	if err := Persist(ctx, db, taskID, &Score{Score: 7, Reasoning: "ok-ish"}, LevelNudge, "stay focused"); err != nil {
		t.Fatal(err)
	}
	// Score 3 → drift_log row.
	if err := Persist(ctx, db, taskID, &Score{Score: 3, Reasoning: "off"}, LevelIntervene, "get back on track"); err != nil {
		t.Fatal(err)
	}

	var stepRows, driftRows int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM steps WHERE task_id = ?`, taskID).Scan(&stepRows)
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM drift_log WHERE task_id = ?`, taskID).Scan(&driftRows)

	if stepRows != 1 {
		t.Errorf("steps rows = %d, want 1", stepRows)
	}
	if driftRows != 1 {
		t.Errorf("drift_log rows = %d, want 1", driftRows)
	}

	// Verify drift_log carries level + correction.
	var level, correction string
	if err := db.QueryRowContext(ctx,
		`SELECT level, COALESCE(correction,'') FROM drift_log WHERE task_id = ?`,
		taskID).Scan(&level, &correction); err != nil {
		t.Fatal(err)
	}
	if level != "intervene" {
		t.Errorf("level=%q, want intervene", level)
	}
	if correction != "get back on track" {
		t.Errorf("correction=%q", correction)
	}
}

func TestPersist_NilDB(t *testing.T) {
	// Should not panic; just no-op.
	if err := Persist(context.Background(), nil, "id", &Score{Score: 1}, LevelHalt, ""); err != nil {
		t.Errorf("err with nil db: %v", err)
	}
}

func TestPersist_NilScore(t *testing.T) {
	if err := Persist(context.Background(), nil, "id", nil, LevelHalt, ""); err != nil {
		t.Errorf("err with nil score: %v", err)
	}
}
