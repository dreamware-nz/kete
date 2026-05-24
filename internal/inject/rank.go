package inject

import (
	"context"

	"github.com/dreamware-nz/kete/internal/store"
)

// RankLocal returns up to n prior tasks for projectPath, ordered
// newest-first. This is the v1 ranker per plan 010 phase 1; smarter
// scoring (recency weighted by access, vector similarity, etc.) lands
// when there's a measurable miss.
func RankLocal(ctx context.Context, db *store.DB, projectPath string, n int) ([]*store.Task, error) {
	tasks, err := db.ListTasks(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	if n > 0 && len(tasks) > n {
		tasks = tasks[:n]
	}
	return tasks, nil
}
