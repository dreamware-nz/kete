package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Task is the canonical reasoning record for one Crush session.
// JSON-serialised columns (Decisions, FilesTouched) are marshalled in
// this layer so callers don't see strings.
type Task struct {
	ID             string
	ProjectPath    string
	UserID         string
	SystemName     string
	Goal           string
	Decisions      []Decision
	FilesTouched   []string
	ReasoningTrace string
	Source         string // "proxy", "jsonl-poll", "manual", ...
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Decision is one captured "we chose X over Y because Z" from a session.
type Decision struct {
	Choice    string `json:"choice"`
	Rationale string `json:"rationale"`
}

// ErrNotFound is returned by GetTask when no row matches.
var ErrNotFound = errors.New("store: not found")

// CreateTask inserts t. The caller must set ID, ProjectPath, Source.
// CreatedAt/UpdatedAt are filled by SQLite defaults if zero.
func (d *DB) CreateTask(ctx context.Context, t *Task) error {
	if t.ID == "" || t.ProjectPath == "" || t.Source == "" {
		return fmt.Errorf("CreateTask: id, project_path, source required")
	}
	decs, err := json.Marshal(t.Decisions)
	if err != nil {
		return fmt.Errorf("marshal decisions: %w", err)
	}
	files, err := json.Marshal(t.FilesTouched)
	if err != nil {
		return fmt.Errorf("marshal files_touched: %w", err)
	}
	_, err = d.ExecContext(ctx, `
		INSERT INTO tasks (
			id, project_path, user_id, system_name, goal,
			decisions, files_touched, reasoning_trace, source
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectPath, nullStr(t.UserID), nullStr(t.SystemName),
		nullStr(t.Goal), string(decs), string(files),
		nullStr(t.ReasoningTrace), t.Source,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// GetTask fetches a task by id.
func (d *DB) GetTask(ctx context.Context, id string) (*Task, error) {
	row := d.QueryRowContext(ctx, taskSelectFields+` FROM tasks WHERE id = ?`, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// UpdateTask updates only the enrichment-time fields. id, project_path,
// source are immutable.
func (d *DB) UpdateTask(ctx context.Context, id string, goal string, decisions []Decision, filesTouched []string, reasoningTrace string) error {
	decs, err := json.Marshal(decisions)
	if err != nil {
		return fmt.Errorf("marshal decisions: %w", err)
	}
	files, err := json.Marshal(filesTouched)
	if err != nil {
		return fmt.Errorf("marshal files_touched: %w", err)
	}
	res, err := d.ExecContext(ctx, `
		UPDATE tasks
		   SET goal = ?, decisions = ?, files_touched = ?,
		       reasoning_trace = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		nullStr(goal), string(decs), string(files), nullStr(reasoningTrace), id)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTasks returns tasks for a project, newest first.
func (d *DB) ListTasks(ctx context.Context, projectPath string) ([]*Task, error) {
	rows, err := d.QueryContext(ctx,
		taskSelectFields+` FROM tasks WHERE project_path = ? ORDER BY created_at DESC`,
		projectPath)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

// ListAllTasks returns every task across every project, newest first.
// Used by `kete status --all`.
func (d *DB) ListAllTasks(ctx context.Context) ([]*Task, error) {
	rows, err := d.QueryContext(ctx,
		taskSelectFields+` FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all tasks: %w", err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

// SearchTasks does a LIKE search over goal + reasoning_trace.
// FTS5 deferred (plan 004 Out of scope).
func (d *DB) SearchTasks(ctx context.Context, query string) ([]*Task, error) {
	pattern := "%" + query + "%"
	rows, err := d.QueryContext(ctx,
		taskSelectFields+` FROM tasks
		   WHERE goal LIKE ? OR reasoning_trace LIKE ?
		   ORDER BY created_at DESC`,
		pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("search tasks: %w", err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

const taskSelectFields = `SELECT id, project_path,
		COALESCE(user_id,''), COALESCE(system_name,''), COALESCE(goal,''),
		COALESCE(decisions,'null'), COALESCE(files_touched,'null'),
		COALESCE(reasoning_trace,''), source, created_at, updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(s scanner) (*Task, error) {
	var (
		t                          Task
		decsJSON, filesJSON        string
		createdStr, updatedStr     string
	)
	if err := s.Scan(
		&t.ID, &t.ProjectPath, &t.UserID, &t.SystemName, &t.Goal,
		&decsJSON, &filesJSON, &t.ReasoningTrace, &t.Source,
		&createdStr, &updatedStr,
	); err != nil {
		return nil, err
	}
	if decsJSON != "" && decsJSON != "null" {
		if err := json.Unmarshal([]byte(decsJSON), &t.Decisions); err != nil {
			return nil, fmt.Errorf("unmarshal decisions: %w", err)
		}
	}
	if filesJSON != "" && filesJSON != "null" {
		if err := json.Unmarshal([]byte(filesJSON), &t.FilesTouched); err != nil {
			return nil, fmt.Errorf("unmarshal files_touched: %w", err)
		}
	}
	t.CreatedAt = parseSQLiteTime(createdStr)
	t.UpdatedAt = parseSQLiteTime(updatedStr)
	return &t, nil
}

func collectTasks(rows *sql.Rows) ([]*Task, error) {
	var out []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func parseSQLiteTime(s string) time.Time {
	// SQLite CURRENT_TIMESTAMP format: "2006-01-02 15:04:05".
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
