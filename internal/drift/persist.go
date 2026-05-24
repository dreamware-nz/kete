package drift

import (
	"context"
	"fmt"

	"github.com/dreamware-nz/kete/internal/store"
)

// Persist writes one drift evaluation to the store. score >= 5 lands
// as a step (real progress); score < 5 lands in drift_log (rejected).
//
// taskID must reference an existing tasks.id row. Returns the
// underlying store error; callers may swallow on the hot path.
func Persist(ctx context.Context, db *store.DB, taskID string, s *Score, level Level, correction string) error {
	if db == nil || s == nil {
		return nil
	}
	if s.Score >= 5 {
		// Step row: encode the action's reasoning + score in `content`.
		_, err := db.ExecContext(ctx,
			`INSERT INTO steps (task_id, seq, role, content)
			 VALUES (?, COALESCE((SELECT MAX(seq)+1 FROM steps WHERE task_id = ?), 1), 'assistant', ?)`,
			taskID, taskID, fmt.Sprintf("score=%d level=%s reasoning=%s", s.Score, level, s.Reasoning))
		return err
	}
	// drift_log: rejected actions, with correction text.
	_, err := db.ExecContext(ctx,
		`INSERT INTO drift_log (task_id, score, level, summary, correction)
		 VALUES (?, ?, ?, ?, ?)`,
		taskID, s.Score, level.String(), s.Reasoning, correction)
	return err
}
