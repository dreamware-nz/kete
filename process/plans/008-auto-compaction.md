---
id: 008-auto-compaction
date: 2026-05-24
status: done
brief: 008-auto-compaction
design: null
adrs: []
---

# 008 — Auto-compaction

> **Status note (2026-05-24):** parts shipped, plan rolled back to
> active because the streaming response-side token-usage tap is not
> yet implemented. Without it the compaction trigger never fires in
> a real session. `compact.Compute`, `Cache`, and `Apply` are all
> wired and tested in isolation. Remaining work: SSE event-stream
> tap that reads `delta.usage` from `event: message_delta` frames as
> they pass through the proxy, accumulates, and calls
> `compactHook.Observe`. Once that's in, set status: done.

## Goal

When a session crosses warning, pre-compute a structured summary; at clear, splice the summary into the next request as the first user message and drop the prior conversation.

## Phases

### Phase 1 — Token-usage tap

- **Outcome:** Per-session counter accumulates `input + output` from each response's `usage` block.
- **Slice:** `internal/compact/usage.go`.
- **Context:** `internal/proxy/forward.go` (plan 002 phase 5–6).
- **Depends-on:** `[]`
- **Done when:** two stub responses → counter accurate.

### Phase 2 — Threshold config

- **Outcome:** `KETE_TOKEN_WARNING_THRESHOLD` (160_000), `KETE_TOKEN_CLEAR_THRESHOLD` (180_000).
- **Slice:** `internal/compact/config.go`.
- **Context:** brief 008 Constraints.
- **Depends-on:** `[]`
- **Done when:** values surface; doctor reports them.

### Phase 3 — Summary struct + parser

- **Outcome:** Typed `Summary{OriginalGoal, Decisions[], Constraints[], CurrentState}`; JSON unmarshal helper.
- **Slice:** `internal/compact/summary.go`.
- **Context:** brief 008 Constraints (summary shape).
- **Depends-on:** `[]`
- **Done when:** snapshot test parses fixture.

### Phase 4 — Pre-compute trigger

- **Outcome:** Crossing warning fires Haiku call; result stored in session state.
- **Slice:** `internal/compact/precompute.go` using `prompts/compact_summary.txt`.
- **Context:** `internal/extract/client.go` (plan 011 phase 1); `internal/compact/usage.go`, `summary.go`, `config.go`.
- **Depends-on:** `[phase-1, phase-2, phase-3]`
- **Done when:** crossing threshold stores a Summary.

### Phase 5 — Synchronous fallback

- **Outcome:** If clear fires before pre-compute completed, run summary inline before forwarding.
- **Slice:** `internal/compact/sync_fallback.go`.
- **Context:** `internal/compact/precompute.go`.
- **Depends-on:** `[phase-4]`
- **Done when:** race test: clear before pre-compute → blocks ~ summary duration.

### Phase 6 — Apply (rewrite request body)

- **Outcome:** Replaces `messages` with `[{role:"user", content:<summary text>}]` plus the new prompt; cleanly through plan 002 phase 9 splice.
- **Slice:** `internal/compact/apply.go`.
- **Context:** `internal/proxy/inject.go`; `internal/compact/summary.go`.
- **Depends-on:** `[phase-3]`
- **Done when:** integration test crossing clear → next body is summary + new prompt.

### Phase 7 — Persist summary into parent task

- **Outcome:** `tasks.reasoning_trace += summary` on apply.
- **Slice:** `internal/compact/persist.go` using `store.UpdateTask` (plan 004 phase 10).
- **Context:** `internal/store/tasks.go`.
- **Depends-on:** `[phase-6]`
- **Done when:** task row contains the summary text after apply.

### Phase 8 — Stderr notification

- **Outcome:** `[kete] compacted at turn N (≈X tokens reclaimed)` to stderr on apply.
- **Slice:** add to `apply.go`.
- **Context:** `internal/compact/apply.go`.
- **Depends-on:** `[phase-6]`
- **Done when:** integration log includes the line.

### Phase 9 — Wire to proxy compaction hook

- **Outcome:** Plan 002 phase 15's no-op consumer becomes pre-compute on warning, apply on clear.
- **Slice:** `internal/compact/hook.go`.
- **Context:** `internal/proxy/compact_hook.go`; `internal/compact/precompute.go`, `apply.go`.
- **Depends-on:** `[phase-4, phase-6]`
- **Done when:** 50-prompt fixture compacts cleanly.

### Phase 10 — Doc: `docs/explanation/auto-compaction.md`

- **Outcome:** Thresholds, summary shape, what's preserved/dropped, cost.
- **Slice:** new file.
- **Context:** brief 008 Doc impact.
- **Depends-on:** `[phase-3]`
- **Done when:** file exists.

## Out of scope

- Native `/compact`. OpenAI shape. User-editable template. Cross-session compaction (plan 010).

## Assumptions

- Plan 011's extractor available. Plan 002's usage tap + hook + body-rewrite helpers in place. Cache prefix loss is the documented cost.
