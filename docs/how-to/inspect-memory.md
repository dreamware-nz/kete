# How-to: inspect captured memory

`~/.kete/memory.db` is plain SQLite. `kete status` and `kete tasks`
cover the common reads, but anything you can write in SQL works.

## Built-in commands

```sh
# Tasks for the current project (cwd, symlinks resolved).
kete status

# Tasks across every project.
kete status --all

# Substring search over goal + reasoning_trace.
kete tasks "auth flow"
```

## Direct SQL

```sh
sqlite3 ~/.kete/memory.db
```

The schema is documented in `docs/reference/schema.md`. The tables
that matter for inspection:

- `tasks` — one row per captured Crush turn. Columns:
  `id, project_path, source, goal, decisions, files_touched,
   reasoning_trace, created_at, updated_at`.
- `steps` — one row per agent action with score >= 5 (real progress).
- `drift_log` — one row per agent action with score < 5 (rejected),
  plus the correction that was queued for the next request.
- `injection_log` — append-only record of which task was injected
  into which request.
- `sync_tracker` — dedupe key for capture sources beyond `proxy`
  (future).

Useful queries:

```sql
-- 5 most recent enriched tasks for the current cwd:
SELECT id, goal, json_array_length(decisions) AS n_decisions, created_at
FROM tasks
WHERE project_path = '/Users/you/some/project'
  AND goal != ''
ORDER BY created_at DESC
LIMIT 5;

-- Drift log for a specific task:
SELECT score, level, summary, correction, created_at
FROM drift_log
WHERE task_id = 'task-abc'
ORDER BY created_at DESC;

-- Which prior tasks have been re-injected, and how often:
SELECT task_id, COUNT(*) AS n_injections
FROM injection_log
GROUP BY task_id
ORDER BY n_injections DESC
LIMIT 20;

-- 8-char short ids the MCP server uses for kete_expand:
-- (computed in Go via inject.ShortID = sha1(id)[:8] hex; SQLite
-- doesn't ship sha1, so this column is computed at read time by
-- the proxy/MCP server, not stored.)
```

## Resetting

```sh
kete purge --yes      # nukes ~/.kete/ entirely
```

`kete purge` without `--yes` prompts y/N. Backup the DB first if
you care about the captured reasoning.

## Before you reach for SQL

Most "where is X?" questions are answered by `kete tasks` or
`kete status --all` plus a grep on the trace column. Reach for
sqlite3 when you need joins (steps + drift_log per task) or counts.
