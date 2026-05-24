---
id: 011-reasoning-extraction
date: 2026-05-24
status: draft
brief: 011-reasoning-extraction
design: null
adrs: [0009-haiku-as-extraction-model]
---

# 011 — Reasoning extraction

## Goal

A small Haiku-backed extractor with embedded prompts ported from the TS implementation, reachable from capture, drift, and compaction.

## Phases

### Phase 1 — `extract.Client`

- **Outcome:** Typed client that POSTs to Anthropic-direct, pinned to `claude-haiku-4-5-20251001` (override via `KETE_DRIFT_MODEL`).
- **Slice:** `internal/extract/client.go`.
- **Context:** ADR 0009; brief 011 Constraints.
- **Depends-on:** `[]`
- **Done when:** unit test against httptest stub returns parsed response.

### Phase 2 — Embedded prompts

- **Outcome:** `prompts/extract_*.txt` + `compact_summary.txt` + `drift_score.txt` + `drift_correct.txt` loadable via `go:embed`.
- **Slice:** `internal/extract/prompts.go` + `prompts/*.txt`.
- **Context:** brief 011 (port from TS).
- **Depends-on:** `[]`
- **Done when:** all named prompts non-empty.

### Phase 3 — Retry/backoff wrapper

- **Outcome:** Exponential backoff on 5xx (max 3 tries); 4xx fail-fast; structured `slog`.
- **Slice:** `internal/extract/retry.go`.
- **Context:** `internal/extract/client.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** 500-then-200 stub succeeds; 400 fails fast.

### Phase 4 — `ExtractTask`

- **Outcome:** `ExtractTask(rawConversation) (goal, decisions, files_touched, error)`.
- **Slice:** `internal/extract/task.go` using prompts + retry.
- **Context:** `internal/extract/client.go`, `internal/extract/prompts.go`, `internal/extract/retry.go`.
- **Depends-on:** `[phase-2, phase-3]`
- **Done when:** fixture conversation produces expected fields.

### Phase 5 — `ExtractDecisions`

- **Outcome:** Sub-call for richer decision rationale.
- **Slice:** `internal/extract/decisions.go`.
- **Context:** `internal/extract/client.go`, `internal/extract/prompts.go`.
- **Depends-on:** `[phase-2, phase-3]`
- **Done when:** fixture asserts ≥ 1 decision with rationale.

### Phase 6 — Cost discipline (`max_tokens` constants)

- **Outcome:** Each call site declares its `max_tokens` matching TS limits.
- **Slice:** `internal/extract/limits.go`.
- **Context:** `internal/extract/task.go`, `decisions.go`.
- **Depends-on:** `[phase-4, phase-5]`
- **Done when:** test asserts request body has `max_tokens ≤ N` per site.

### Phase 7 — Wire from capture

- **Outcome:** Plan 006 phase 4's pipeline calls `ExtractTask` after each raw row write; `UpdateTask` enriches.
- **Slice:** wire in `internal/capture/pipeline.go`.
- **Context:** `internal/capture/pipeline.go`; `internal/extract/task.go`; `internal/store/tasks.go` (plan 004 phase 10).
- **Depends-on:** `[phase-4]`
- **Done when:** 1 request → 1 raw row → enriched within ~1s.

### Phase 8 — Wire from drift

- **Outcome:** Plan 007 phases 1 + 6 call into this client.
- **Slice:** import and inject.
- **Context:** `internal/drift/score.go`, `internal/drift/correct.go`.
- **Depends-on:** `[phase-1, phase-2]`
- **Done when:** drift fixtures pass with this client.

### Phase 9 — Wire from compaction

- **Outcome:** Plan 008 phases 3 + 5 call into this client.
- **Slice:** import and inject.
- **Context:** `internal/compact/precompute.go`, `internal/compact/sync_fallback.go`.
- **Depends-on:** `[phase-1, phase-2]`
- **Done when:** compaction integration test passes.

### Phase 10 — Doc: `docs/explanation/extraction.md`

- **Outcome:** Per-call-site description: what's extracted, prompt used, failure mode.
- **Slice:** new file.
- **Context:** brief 011 Doc impact.
- **Depends-on:** `[phase-4, phase-5]`
- **Done when:** file exists; cited from briefs 006, 007, 008.

## Out of scope

- Different model. Caching extraction outputs. Prompt-engineering tooling. Streaming.

## Assumptions

- Anthropic-direct is reachable; `ANTHROPIC_API_KEY` is set even when user upstream is Bedrock or cc-proxy.
- Ported TS prompts produce equivalent JSON.
