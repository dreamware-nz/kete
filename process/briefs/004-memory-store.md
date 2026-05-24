---
id: 004-memory-store
date: 2026-05-24
status: accepted
from-idea: 2026-05-24-kete
design: null
adrs: [0002-sqlite-via-modernc-no-cgo, 0003-clean-numbered-migrations, 0004-kete-dotdir-layout-and-perms]
plan: 004-memory-store
---

# 004 — Memory store

## Problem

The local SQLite database at `~/.kete/memory.db` is the only persistent state kete has. Everything else — the proxy, the MCP server, the (future) cloud sync — is a thin layer over it. We use `modernc.org/sqlite` (pure-Go, no cgo; ADR 0002), enable WAL with a `wal_checkpoint(TRUNCATE)` at startup, and own the schema outright (ADR 0000 — no wire-compat with TS-grov).

The core tables: `tasks`, `session_states`, `steps`, `drift_log`. (`file_reasoning` from grov is dropped unless brief-stage need surfaces it.) Migrations use a numbered `schema_migrations` table with up-only steps (ADR 0003). The schema is ours; we evolve it the way clean Go projects do.

## Who is hurt

The future kete user whose schema doesn't migrate cleanly on a release upgrade. That's why we own the migration story instead of inheriting one from another project.

## Constraints

- File path: `~/.kete/memory.db`. Override via `KETE_DB_PATH` (used by tests).
- Directory mode `0700`, file mode `0600`. Created if missing. (ADR 0004.)
- WAL journal mode + `wal_checkpoint(TRUNCATE)` at startup.
- Migration discipline: `schema_migrations` table tracks applied versions; up-only steps in `internal/store/migrations/`; embedded via `go:embed`. (ADR 0003.)
- Foreign keys enabled (`PRAGMA foreign_keys = ON`). We aren't compat-bound to grov's choice not to.
- Single connection, opened lazily, closed in the CLI's deferred cleanup. Multiple kete processes can open the DB concurrently (WAL); we don't assume exclusivity (e.g. proxy and MCP server are separate processes).

## Success looks like

- A fresh kete install creates `~/.kete/memory.db` with file mode `0600` and the parent dir at `0700`.
- A `tasks` row written by the proxy is visible from `kete tasks` and from the MCP server's `kete_preview` query within the same SQLite WAL window.
- A schema upgrade to a new kete release applies cleanly: the `schema_migrations` table records the new version; existing data is preserved; the proxy and MCP server come up against the upgraded DB.
- A test against `KETE_DB_PATH=$(mktemp).db` exercises the full migration sequence from empty to current.

## Non-goals

- Down-migrations. Up-only is the discipline; rollback is "restore from backup".
- A heavyweight migration framework (atlas, flyway-style declarative diffs). A `schema_migrations` table plus embedded `.sql` files is enough.
- Switching to `mattn/go-sqlite3` for performance. It needs cgo, which breaks static cross-compilation. ADR 0002.
- Sharded / multi-file storage. One file per machine.
- Encryption at rest. `0600` perms is the bar.

## Open questions

All settled in existing ADRs:

- ADR 0002 — `modernc.org/sqlite` (pure Go, no cgo).
- ADR 0003 — clean numbered migrations.
- ADR 0004 — `~/.kete/` layout and modes.

Schema specifics — exact column list per table, JSON shape conventions, indexes — are an implementation detail of the plan, not the brief.

## Doc impact

- `docs/reference/schema.md` `[new]` — generated from a `go run ./cmd/dump-schema`.
- `docs/explanation/migrations.md` `[new]` — schema_migrations table, up-only discipline.
- `[none]` for changelog beyond the initial release entry.
