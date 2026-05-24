---
id: 001-cli-shell
date: 2026-05-24
status: draft
brief: 001-cli-shell
design: null
adrs: [0010-cli-framework]
---

# 001 — CLI shell

## Goal

A `kete` cobra binary with the v1 subcommand surface, a single error path that always closes the DB, and a `--version` wired to ldflags.

## Phases

### Phase 1 — `cmd/kete/main.go` skeleton

- **Outcome:** `go build -o kete ./cmd/kete` works; `kete --help` runs; bad command exits 1 with `Error: …`.
- **Slice:** root cobra command; `run()`-returning-`error`; `os.Exit(1)` on error.
- **Context:** ADR 0010.
- **Depends-on:** `[]`
- **Done when:** `kete badcmd` prints `Error: …`, exits 1.

### Phase 2 — `--version` from ldflags

- **Outcome:** `kete --version` prints from `-X main.version=…`.
- **Slice:** `var version = "dev"`; cobra `Version` field.
- **Context:** `cmd/kete/main.go`.
- **Done when:** `go build -ldflags "-X main.version=0.1.0"` → `kete --version` → `0.1.0`.

### Phase 3 — `withStore` helper

- **Outcome:** A single helper opens the store, calls the handler, closes deferred.
- **Slice:** `cmd/kete/store.go` with `withStore(fn func(*store.Store) error) error`.
- **Context:** `cmd/kete/main.go`; `internal/store` API (plan 004 phase 13).
- **Depends-on:** `[phase-1]`
- **Done when:** unit test panics inside fn, store still closed (asserted via `lsof` or close hook).

### Phase 4 — `kete proxy` stub

- **Outcome:** Subcommand exists, parses `--debug`, `--extended-cache`, prints "not yet implemented".
- **Slice:** `cmd/kete/proxy.go`.
- **Context:** `cmd/kete/main.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** flags parse; exits 0.

### Phase 5 — `kete mcp` stub

- **Outcome:** Subcommand exists; reads stdin, exits clean.
- **Slice:** `cmd/kete/mcp.go`.
- **Context:** `cmd/kete/main.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** `echo "" | kete mcp` exits 0.

### Phase 6 — `kete status`

- **Outcome:** Lists tasks for cwd (or `--all`); under 50 ms warm.
- **Slice:** `cmd/kete/status.go`; uses `withStore` + `ListTasks(cwd-normalised)`.
- **Context:** `cmd/kete/store.go`; `internal/store/tasks.go` (plan 004 phase 11).
- **Depends-on:** `[phase-3]`
- **Done when:** seeded DB → correct output; `time kete status` < 50 ms.

### Phase 7 — `kete tasks <query>` search

- **Outcome:** Calls `SearchTasks` and prints matches.
- **Slice:** `cmd/kete/tasks.go`.
- **Context:** `cmd/kete/store.go`; `internal/store/tasks.go` (plan 004 phase 12).
- **Depends-on:** `[phase-3]`
- **Done when:** seeded keyword hits.

### Phase 8 — `kete drift-test` stub

- **Outcome:** Parses `<prompt> --goal <goal>`; prints "not yet wired".
- **Slice:** `cmd/kete/drifttest.go`.
- **Context:** `cmd/kete/main.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** missing `--goal` errors; valid args exit 0.

### Phase 9 — `kete doctor` core checks

- **Outcome:** Sanity-checks `~/.kete/` exists with `0700`; prints PASS/FAIL.
- **Slice:** `cmd/kete/doctor.go`; one `Check` struct per row.
- **Context:** ADR 0004; `internal/store/path.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** PASS row on healthy install; FAIL when perms wrong.

### Phase 10 — `kete doctor` upstream reachability

- **Outcome:** HEAD-ping the configured upstream URL; PASS/FAIL row.
- **Slice:** extend `cmd/kete/doctor.go`.
- **Context:** `cmd/kete/doctor.go`; brief 002 routing rules.
- **Depends-on:** `[phase-9]`
- **Done when:** stopped upstream → FAIL with clear error.

### Phase 11 — `kete purge` with confirmation

- **Outcome:** Deletes `~/.kete/` after y/N; `--yes` for non-interactive.
- **Slice:** `cmd/kete/purge.go`.
- **Context:** `cmd/kete/main.go`; `internal/store/path.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** `--yes` removes a tempdir; manual y/N tested.

### Phase 12 — Generated `docs/reference/cli.md`

- **Outcome:** Reference doc generated from cobra help.
- **Slice:** `make docs` target via cobra `GenMarkdownTree` or hand walk.
- **Context:** all `cmd/kete/*.go`.
- **Depends-on:** `[phase-4, phase-5, phase-6, phase-7, phase-8, phase-9, phase-11]`
- **Done when:** file exists with all subcommands.

### Phase 13 — README quickstart

- **Outcome:** 5-line quickstart at top of `README.md`.
- **Slice:** `README.md`.
- **Context:** brief 000 Doc impact.
- **Depends-on:** `[]`
- **Done when:** fresh reader can run kete from it.

## Out of scope

- `kete init`, completions, coloured output, cloud-sync subcommands.

## Assumptions

- cobra v1.x stable; 50 ms cold-start achievable with cobra + modernc.
