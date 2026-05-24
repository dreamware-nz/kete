---
id: 004-memory-store
date: 2026-05-24
status: done
brief: 004-memory-store
design: null
adrs: [0002-sqlite-via-modernc-no-cgo, 0003-clean-numbered-migrations, 0004-kete-dotdir-layout-and-perms]
---

# 004 — Memory store

## Goal

A pure-Go SQLite store at `~/.kete/memory.db` that owns kete's only durable state, with clean numbered migrations, WAL mode, and a small typed API every other component uses.

## Phases

### Phase 1 — Repo skeleton

- **Outcome:** `go.mod` + empty `internal/store` package; `go build ./...` green.
- **Slice:** `go mod init github.com/dreamware-nz/kete`; pull `modernc.org/sqlite`; stub `internal/store/store.go`.
- **Context:** ADR 0001, ADR 0002.
- **Depends-on:** `[]`
- **Done when:** `go build ./...` green; `go test ./...` is a no-op pass.
- **Risks:** Go version mismatch with ADR 0001.

### Phase 2 — Dotdir + path resolution

- **Outcome:** `~/.kete/` created on first call with `0700`; `KETE_HOME` override honoured.
- **Slice:** `internal/store/path.go` with `DefaultDir()`, `Open(path string)` (Open delegates to phases 3–4 later; for now just resolves and creates dir).
- **Context:** ADR 0004; `internal/store/store.go`.
- **Done when:** unit test creates a tempdir via `KETE_HOME`, asserts perms `0700`.
- **Risks:** Windows perms (out of scope).

### Phase 3 — `Open` opens DB + applies pragmas

- **Outcome:** `Open` returns a `*sql.DB` with `journal_mode=WAL`, `synchronous=NORMAL`, runs `PRAGMA wal_checkpoint(TRUNCATE)`.
- **Slice:** `applyPragmas(*sql.DB)`; called from `Open`.
- **Context:** `internal/store/path.go`, `internal/store/store.go`; ADR 0002.
- **Done when:** test inspects `journal_mode` after `Open`, asserts `wal`.

### Phase 4 — Migration runner

- **Outcome:** `migrations/NNNN_*.sql` files run in order; `schema_migrations(version, applied_at)` tracks state; idempotent.
- **Slice:** `internal/store/migrate.go` with `Migrate(db, fs.FS)`; `go:embed migrations/*.sql`.
- **Context:** `internal/store/store.go`; ADR 0003.
- **Done when:** test runs migrate twice, asserts no duplicate rows.
- **Risks:** filename sort bugs; pin `sort.Strings`.

### Phase 5 — `tasks` table migration `0001`

- **Outcome:** First migration creates `tasks` per ADR 0003 shape.
- **Slice:** `migrations/0001_tasks.sql` — `id TEXT PK, project_path, user_id, system_name, goal, decisions JSON, files_touched JSON, reasoning_trace TEXT, source TEXT, created_at, updated_at`.
- **Context:** brief 004; ADR 0003; phase 4's runner.
- **Done when:** `Migrate` creates the table; `PRAGMA table_info(tasks)` matches.

### Phase 6 — `steps` + `drift_log` tables migration `0002`

- **Outcome:** Per brief 007 schema with FK to `tasks(id)`.
- **Slice:** `migrations/0002_drift.sql`.
- **Context:** brief 007 Constraints; ADR 0011; phase 5.
- **Done when:** tables exist; FK validated in test.

### Phase 7 — `sync_tracker` table migration `0003`

- **Outcome:** `sync_tracker(source TEXT, key TEXT, captured_at, PRIMARY KEY(source,key))`.
- **Slice:** `migrations/0003_sync_tracker.sql`.
- **Context:** brief 006 Constraints.
- **Depends-on:** `[phase-4]`
- **Done when:** `INSERT OR IGNORE` is idempotent in test.

### Phase 8 — `Task` struct + `CreateTask`

- **Outcome:** Typed insert path; JSON cols marshalled in the layer.
- **Slice:** `internal/store/tasks.go` with `Task`, `CreateTask(ctx, *Task)`.
- **Context:** `internal/store/store.go`; phase 5 schema.
- **Done when:** unit test inserts and DB row matches.

### Phase 9 — `GetTask`

- **Outcome:** Round-trip retrieval of a single task by id.
- **Slice:** `GetTask(ctx, id) (*Task, error)`.
- **Context:** `internal/store/tasks.go`; phase 8.
- **Done when:** create → get → equal.

### Phase 10 — `UpdateTask` for extraction enrichment

- **Outcome:** Allow filling `goal`, `decisions`, `files_touched` after creation.
- **Slice:** `UpdateTask(ctx, id, fields ...)` with explicit settable fields.
- **Context:** `internal/store/tasks.go`; phase 8.
- **Depends-on:** `[phase-8]`
- **Done when:** test creates raw, updates, re-fetches and matches.

### Phase 11 — `ListTasks(projectPath)`

- **Outcome:** Tasks for a project ordered `created_at DESC`.
- **Slice:** `ListTasks` query in `tasks.go`.
- **Context:** `internal/store/tasks.go`; phase 8.
- **Depends-on:** `[phase-8]`
- **Done when:** seed 3 tasks across 2 projects, returns correct subset.

### Phase 12 — `SearchTasks(query)`

- **Outcome:** LIKE search over goal/keywords (no FTS5 yet).
- **Slice:** `SearchTasks` query in `tasks.go`.
- **Context:** `internal/store/tasks.go`; phase 8.
- **Depends-on:** `[phase-8]`
- **Done when:** seeded keyword returns the matching row.

### Phase 13 — `Close` + WAL truncate

- **Outcome:** Clean shutdown leaves only `memory.db`.
- **Slice:** `Close()` runs `wal_checkpoint(TRUNCATE)` then `db.Close()`.
- **Context:** `internal/store/store.go`; phase 3.
- **Done when:** test asserts no orphan `-wal`/`-shm` after Close.

### Phase 14 — Doc: `docs/reference/schema.md`

- **Outcome:** Schema documented column-by-column; one section per table.
- **Slice:** new file; hand-written from migrations 0001–0003.
- **Context:** `migrations/*.sql`; brief 004 Doc impact.
- **Depends-on:** `[phase-5, phase-6, phase-7]`
- **Done when:** file exists; linked from README.

## Out of scope

- FTS5 (defer until injection demands it).
- Cloud-sync columns (brief 005, deferred).
- Cross-platform perms beyond macOS/Linux.

## Assumptions

- `modernc.org/sqlite` performance is adequate for v1 single-user.
- WAL handles every concurrent-process case we need.
- The schema set in phases 5–7 is enough for briefs 002, 003, 006, 007, 010, 011.
