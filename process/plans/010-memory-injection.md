---
id: 010-memory-injection
date: 2026-05-24
status: active
brief: 010-memory-injection
design: null
adrs: []
---

# 010 — Memory injection

## Goal

Inject relevant prior tasks into a new request before the cache breakpoint, with `kete_expand` letting the model pull more on demand.

## Phases

### Phase 1 — Local ranker

- **Outcome:** `inject.RankLocal(projectPath, n) []*Task` by `created_at DESC`.
- **Slice:** `internal/inject/rank.go`.
- **Context:** `internal/store/tasks.go` (plan 004 phase 11).
- **Depends-on:** `[]`
- **Done when:** seed 5 → asserts ordering and limit.

### Phase 2 — Preview format

- **Outcome:** `inject.Preview(t *Task) string` produces a short text block.
- **Slice:** `internal/inject/format.go`.
- **Context:** brief 010.
- **Depends-on:** `[]`
- **Done when:** snapshot test of a fixed task.

### Phase 3 — Injection log migration

- **Outcome:** `injection_log(task_id, project_path, request_id, created_at)` table.
- **Slice:** `migrations/0004_injection_log.sql`.
- **Context:** ADR 0003; brief 010 Constraints.
- **Depends-on:** `[]`
- **Done when:** `Migrate` creates the table.

### Phase 4 — `addInjectionRecord`

- **Outcome:** Append-only row per injection; non-blocking goroutine.
- **Slice:** `internal/inject/log.go`.
- **Context:** `migrations/0004_injection_log.sql`; `internal/store/store.go`.
- **Depends-on:** `[phase-3]`
- **Done when:** seeded injection → row exists.

### Phase 5 — Project-path normalisation reuse

- **Outcome:** Both write side (capture) and read side (injection) call `capture.NormaliseProject`.
- **Slice:** drop any inline normaliser; import from `internal/capture/path.go`.
- **Context:** `internal/capture/path.go` (plan 006 phase 3).
- **Depends-on:** `[]`
- **Done when:** unit test asserts same key on both sides.

### Phase 6 — Shared 8-char preview cache

- **Outcome:** In-memory `id8 → fullTask` map shared by proxy injection and MCP `kete_expand`.
- **Slice:** `internal/inject/cache.go`.
- **Context:** `internal/inject/format.go`; brief 010.
- **Depends-on:** `[phase-2]`
- **Done when:** id from proxy resolves via expand in same process.

### Phase 7 — Wire into proxy injection middleware

- **Outcome:** Plan 002 phase 13's "newest" placeholder is replaced with `RankLocal` + `Preview`, splice via plan 002 phase 10; record via `addInjectionRecord`.
- **Slice:** edit `internal/proxy/inject_mw.go`.
- **Context:** `internal/proxy/inject_mw.go`; `internal/inject/rank.go`, `format.go`, `cache.go`, `log.go`.
- **Depends-on:** `[phase-1, phase-2, phase-4, phase-5, phase-6]`
- **Done when:** integration test: top 3 previews land; prefix bytes still cache-stable.

### Phase 8 — Wire `kete_expand` into shared cache

- **Outcome:** Plan 003 phase 6's expand reads from `inject.Cache`.
- **Slice:** edit `internal/mcp/expand.go` to consult shared cache.
- **Context:** `internal/mcp/expand.go`; `internal/inject/cache.go`.
- **Depends-on:** `[phase-6]`
- **Done when:** end-to-end: proxy injects 3 previews; MCP client expands one to full task.

### Phase 9 — Doc: `docs/explanation/memory-injection.md`

- **Outcome:** What's injected, ranking, expand cycle, cache breakpoint rule.
- **Slice:** new file.
- **Context:** brief 010 Doc impact.
- **Depends-on:** `[phase-1, phase-2]`
- **Done when:** file exists.

## Out of scope

- Cloud ranking. Embedding-based search. Cross-project injection. User-facing "show injection" CLI.

## Assumptions

- Plan 002's injection helpers stable. Local ranking by `created_at` adequate for v1. Plan 003's MCP server in place.
