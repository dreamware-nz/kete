package inject

import (
	"context"

	"github.com/dreamware-nz/kete/internal/store"
)

// LogInjection appends an injection_log row. Best-effort; errors are
// returned for callers that care, but the proxy hot path swallows.
func LogInjection(ctx context.Context, db *store.DB, taskID, projectPath, requestID string) error {
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO injection_log (task_id, project_path, request_id) VALUES (?, ?, ?)`,
		taskID, projectPath, requestID)
	return err
}
