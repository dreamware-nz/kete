---
number: 0003
title: Schema migrations are numbered, up-only, embedded SQL files
date: 2026-05-24
status: accepted
brief: 004-memory-store
supersedes: null
superseded-by: null
---

# 0003 — Schema migrations are numbered, up-only, embedded SQL files

## Context

kete owns its schema; there are no existing users or upstream wire to keep compatible with. The shape of the choice for any clean Go project shipping a SQLite store is whether to track schema versions explicitly or accumulate idempotent ALTERs.

"No migrations framework, just additive idempotent ALTERs" is a discipline that earns its place when there are paying users on every historical state of the schema and the team has chosen never to break them. We are not in that situation. With no compat constraint, idempotent-ALTER becomes a vice rather than a virtue: it hides version state, makes "what shipped at v0.3.1" unanswerable from the DB itself, forces every developer to read the migrations function top-to-bottom to understand current shape, and silently swallows real errors when the catch is too broad.

## Decision

kete uses a numbered migrations system:

- A `schema_migrations` table tracks applied migrations: `(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`.
- Migrations live in `internal/store/migrations/` as `NNNN_description.sql` files (e.g. `0001_initial.sql`, `0002_add_drift_log.sql`). They are embedded into the binary via `go:embed`.
- Migrations are **up-only**. No down migrations. Rollback is "restore from backup".
- On startup, `store.Migrate()` reads `schema_migrations`, finds unapplied versions, applies them in order inside transactions, records each in `schema_migrations`. Failure aborts startup with the failing version and SQL surfaced.
- Each migration is a single `.sql` file containing one or more statements separated by `;`. Comments allowed; transactions managed by the caller, not embedded in the SQL.
- The schema is described by replaying all migrations from empty. There is no canonical `schema.sql` file; the migration set *is* the schema.

## Options considered

- **Numbered up-only migrations (this ADR).** Standard, honest, version-tracked, easy to read.
- **`golang-migrate/migrate`.** Same shape; brings a dependency and CLI surface we don't need (kete is the only consumer). Reject; the embedded approach is ~50 lines of Go.
- **`goose`.** Ditto.
- **Atlas / declarative diff.** Compares desired schema to current and emits a diff. Powerful for projects with many environments; overkill here. Reject.
- **Idempotent ALTERs without a migrations table.** What grov-the-product does because it has historical users. Hides version state; broad try/catch hides errors. Reject.

## Consequences

Easier:

- A user can `SELECT * FROM schema_migrations` and answer "what state am I in".
- `kete doctor` can detect "DB is older than this binary" and prompt for upgrade rather than hitting a column-not-found at runtime.
- Test fixtures use the same migrations the production code does. A test DB created at `0005` and upgraded to `0008` exercises real migration paths.
- Code review of a schema change is "look at the new `.sql` file", not "read the body of an accreting migration function".
- Errors propagate cleanly. There's no broad try/catch hiding "duplicate column" alongside "table is locked".

Harder:

- Renames / drops still cost a one-off "shadow table + copy + swap" migration. Same as before. The discipline is "additive when possible, destructive when necessary, always recorded".
- A botched migration that ran halfway needs hand-recovery. We document "migrations run in transactions; partial application means rollback at the SQLite level, leaving `schema_migrations` unchanged". On extreme corruption: restore from backup.
- We commit to never editing a migration file once it's been released. Edits create silent divergence (already-upgraded users have one shape; fresh installs have another). Code review must enforce.
