---
id: 006-capture-pipelines
date: 2026-05-24
status: active
brief: 006-capture-pipelines
design: null
adrs: [0014-jsonl-session-poll-vs-hooks]
---

# 006 — Capture pipelines

## Goal

A single store-API write path reachable from every capture source, with a unified `sync_tracker` and capture never blocking the request path.

## Phases

### Phase 1 — `capture.Source` interface + registry

- **Outcome:** `Source` (`Name() string`, `Run(ctx) error`); `Registry.Register/Get`.
- **Slice:** `internal/capture/source.go`.
- **Context:** brief 006 Constraints.
- **Depends-on:** `[]`
- **Done when:** registry round-trip test.

### Phase 2 — `sync_tracker` API

- **Outcome:** `tracker.Seen(source, key) bool` and `tracker.Mark(source, key)`.
- **Slice:** `internal/capture/tracker.go`.
- **Context:** `internal/store/store.go`; plan 004 phase 7.
- **Depends-on:** `[]`
- **Done when:** second `Seen` returns true after `Mark`.

### Phase 3 — Project-path normalisation

- **Outcome:** `capture.NormaliseProject(path) string` — stable key (basename or absolute).
- **Slice:** `internal/capture/path.go`.
- **Context:** brief 003, brief 010 (must match).
- **Depends-on:** `[]`
- **Done when:** symlink, trailing slash, relative path covered.

### Phase 4 — Pipeline + worker

- **Outcome:** `Pipeline.Submit(*Task)` is async; bounded buffered channel; one worker goroutine writes via `store.CreateTask`.
- **Slice:** `internal/capture/pipeline.go`.
- **Context:** `internal/store/tasks.go`; `internal/capture/tracker.go`.
- **Depends-on:** `[phase-2]`
- **Done when:** 10 submits → 10 rows; submit returns immediately.

### Phase 5 — Refactor proxy capture to use the pipeline

- **Outcome:** Plan 002 phase 11's goroutine becomes `Pipeline.Submit`.
- **Slice:** edit `internal/proxy/capture.go`.
- **Context:** `internal/proxy/capture.go`; `internal/capture/pipeline.go`.
- **Depends-on:** `[phase-4]`
- **Done when:** existing proxy capture tests still pass; rows still land.

### Phase 6 — Antigravity scanner

- **Outcome:** Polls Antigravity JSONL; emits new chats; dedup via `sync_tracker(source="antigravity")`.
- **Slice:** `internal/capture/antigravity.go`.
- **Context:** ADR 0014; `internal/capture/source.go`, `tracker.go`, `pipeline.go`.
- **Depends-on:** `[phase-1, phase-4]`
- **Done when:** 2-session fixture yields 2 tasks; rerun yields 0.

### Phase 7 — Cursor IDE source

- **Outcome:** Polls Cursor chat history; same dedup pattern.
- **Slice:** `internal/capture/cursor.go`.
- **Context:** `internal/capture/source.go`, `tracker.go`, `pipeline.go`.
- **Depends-on:** `[phase-1, phase-4]`
- **Done when:** fixture yields tasks; rerun idempotent.

### Phase 8 — Claude Code source

- **Outcome:** Reads `~/.claude/projects/<slug>/sessions/*.jsonl`.
- **Slice:** `internal/capture/claudecode.go`.
- **Context:** `internal/capture/source.go`, `tracker.go`, `pipeline.go`.
- **Depends-on:** `[phase-1, phase-4]`
- **Done when:** fixture yields tasks.

### Phase 9 — Crush source

- **Outcome:** Reads Crush session DBs.
- **Slice:** `internal/capture/crush.go`.
- **Context:** `internal/capture/source.go`, `tracker.go`, `pipeline.go`; Crush data layout.
- **Depends-on:** `[phase-1, phase-4]`
- **Done when:** fixture yields tasks.

### Phase 10 — Failure logging contract

- **Outcome:** Each source's `Run` wraps panics + errors with structured `slog`; pipeline keeps draining.
- **Slice:** `internal/capture/run.go` with `runSource(s Source)` shell.
- **Context:** `internal/capture/source.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** synthetic broken JSONL → log line + pipeline continues.

### Phase 11 — Wire sources into `kete mcp`

- **Outcome:** `kete mcp` starts non-proxy sources in goroutines; `--no-scanners` disables for tests.
- **Slice:** edit `cmd/kete/mcp.go`.
- **Context:** `cmd/kete/mcp.go`; `internal/capture/source.go`.
- **Depends-on:** `[phase-6, phase-7, phase-8, phase-9, phase-10]`
- **Done when:** fixtures populate DB after launch.

### Phase 12 — Doc: `docs/explanation/capture-sources.md`

- **Outcome:** One section per source: what it reads, dedup, silent failures.
- **Slice:** new file.
- **Context:** brief 006 Doc impact.
- **Depends-on:** `[phase-6, phase-7, phase-8, phase-9]`
- **Done when:** file exists.

## Out of scope

- Hooks-based capture. New sources beyond the five. Real-time push.

## Assumptions

- Source file formats stable for v1; 60 s poll acceptable. `sync_tracker` shape suffices.
