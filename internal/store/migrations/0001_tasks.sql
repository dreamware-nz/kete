CREATE TABLE tasks (
    id              TEXT PRIMARY KEY,
    project_path    TEXT NOT NULL,
    user_id         TEXT,
    system_name     TEXT,
    goal            TEXT,
    decisions       TEXT,
    files_touched   TEXT,
    reasoning_trace TEXT,
    source          TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX tasks_project_created ON tasks(project_path, created_at DESC);
