# Schema reference

The kete database lives at `~/.kete/memory.db` (override with
`KETE_DB_PATH`, or relocate the whole dotdir with `KETE_HOME`). It is a
single pure-Go SQLite database (ADR 0002) opened in WAL mode with
`synchronous=NORMAL` and foreign keys on.

Schema versions are tracked in `schema_migrations(version, applied_at)`.
Migrations are up-only, embedded in the binary at build time, and
applied transactionally on every `Open` (ADR 0003).

## `tasks` (migration 0001)

One row per captured Crush session. The unit of memory.

| Column            | Type     | Notes                                                     |
|-------------------|----------|-----------------------------------------------------------|
| `id`              | TEXT PK  | Caller-assigned session id.                               |
| `project_path`    | TEXT     | Absolute path of the working directory.                   |
| `user_id`         | TEXT     | Optional. The local OS user.                              |
| `system_name`     | TEXT     | Optional. The host the session ran on.                    |
| `goal`            | TEXT     | One-line goal extracted from the session.                 |
| `decisions`       | TEXT     | JSON array of `{choice, rationale}`.                      |
| `files_touched`   | TEXT     | JSON array of repo-relative paths.                        |
| `reasoning_trace` | TEXT     | Free-form trace assembled by the extractor.               |
| `source`          | TEXT     | `"proxy"`, `"jsonl-poll"`, `"manual"`, …                  |
| `created_at`      | TEXT     | `CURRENT_TIMESTAMP` on insert.                            |
| `updated_at`      | TEXT     | `CURRENT_TIMESTAMP`; bumped by `UpdateTask`.              |

Index: `tasks(project_path, created_at DESC)` — backs `ListTasks`.

## `steps` and `drift_log` (migration 0002)

`steps` holds the ordered conversation per task; `drift_log` holds
per-turn drift scores with corrections (ADR 0011, brief 007).

`steps`:

| Column       | Type    | Notes                                              |
|--------------|---------|----------------------------------------------------|
| `id`         | INTEGER | autoincrement PK.                                  |
| `task_id`    | TEXT FK | `tasks(id) ON DELETE CASCADE`.                     |
| `seq`        | INTEGER | Per-task monotonic step number.                    |
| `role`       | TEXT    | `"user"`, `"assistant"`, `"tool"`, …               |
| `content`    | TEXT    | Verbatim message body.                             |
| `created_at` | TEXT    | `CURRENT_TIMESTAMP`.                               |

Index: `steps(task_id, seq)`.

`drift_log`:

| Column       | Type    | Notes                                                     |
|--------------|---------|-----------------------------------------------------------|
| `id`         | INTEGER | autoincrement PK.                                         |
| `task_id`    | TEXT FK | `tasks(id) ON DELETE CASCADE`.                            |
| `score`      | INTEGER | 1–10 from the drift extractor.                            |
| `level`      | TEXT    | `"info"`, `"warn"`, `"correct"`, `"abort"`.               |
| `summary`    | TEXT    | One-line description of the drift.                        |
| `correction` | TEXT    | The injected correction text.                             |
| `created_at` | TEXT    | `CURRENT_TIMESTAMP`.                                      |

Index: `drift_log(task_id, created_at DESC)`.

## `sync_tracker` (migration 0003)

Per-source dedup. A capture pipeline records `(source, key)` on every
ingest; `INSERT OR IGNORE` makes ingest idempotent.

| Column        | Type | Notes                                                 |
|---------------|------|-------------------------------------------------------|
| `source`      | TEXT | `"jsonl-poll"`, `"manual"`, …                         |
| `key`         | TEXT | Source-specific natural key (file path, hash, …).     |
| `captured_at` | TEXT | `CURRENT_TIMESTAMP`.                                  |

Primary key: `(source, key)`.

## Out of scope (v1)

- FTS5 over `tasks` — not until injection demands it.
- Cloud-sync columns — brief 005, deferred (ADR 0016).
- Down migrations — recover from backup (ADR 0003).
