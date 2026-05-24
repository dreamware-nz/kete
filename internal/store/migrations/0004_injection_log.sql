-- Plan 010 phase 3: append-only log of memory injections.
-- One row per (task injected, request that injected it).
CREATE TABLE injection_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id      TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    project_path TEXT NOT NULL,
    request_id   TEXT,
    created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX injection_log_task ON injection_log(task_id, created_at DESC);
CREATE INDEX injection_log_project ON injection_log(project_path, created_at DESC);
