---
id: 007-anti-drift
date: 2026-05-24
status: done
brief: 007-anti-drift
design: null
adrs: [0011-drift-correction-four-levels]
---

# 007 — Anti-drift

## Goal

Score each agent action 1–10 against the session goal via Haiku, escalate or recover, and inject the right correction at the right level inline in the next request.

## Phases

### Phase 1 — Fixture set

- **Outcome:** ≥ 20 hand-labelled fixture sessions in `testdata/drift/` covering all four levels.
- **Slice:** `testdata/drift/*.json` with `(goal, action, expected_level)`.
- **Context:** brief 007 Success-looks-like.
- **Depends-on:** `[]`
- **Done when:** loader test enumerates ≥ 20 entries.

### Phase 2 — `drift.Score`

- **Outcome:** `drift.Score(goal, action) (int, string)` via Haiku with embedded score-prompt.
- **Slice:** `internal/drift/score.go`; uses `prompts/drift_score.txt` (plan 011 phase 2).
- **Context:** ADR 0011; `internal/extract/client.go` (plan 011 phase 1); `prompts/drift_score.txt`.
- **Depends-on:** `[phase-1]`
- **Done when:** fixtures produce deterministic scores within tolerance.

### Phase 3 — `drift.Level`

- **Outcome:** `Level(score) → none|nudge|correct|intervene|halt`.
- **Slice:** `internal/drift/level.go`.
- **Context:** brief 007 Constraints (level table).
- **Depends-on:** `[]`
- **Done when:** unit test covers all bands.

### Phase 4 — Persist `steps` / `drift_log`

- **Outcome:** `score ≥ 5` writes `steps`; `< 5` writes `drift_log`.
- **Slice:** `internal/drift/persist.go`; uses store (plan 004 phase 6).
- **Context:** brief 007 Constraints; `migrations/0002_drift.sql`.
- **Depends-on:** `[phase-2, phase-3]`
- **Done when:** test asserts correct table for each band.

### Phase 5 — Escalation counter

- **Outcome:** Per-session counter; +1 on correction, −1 on `score ≥ 8`, never < 0.
- **Slice:** `internal/drift/state.go`.
- **Context:** brief 007 Constraints (escalation rules).
- **Depends-on:** `[phase-3]`
- **Done when:** sequence test: nudge, nudge, recover → 1.

### Phase 6 — Forced-recovery threshold

- **Outcome:** `state.ForcedRecovery() bool` true at escalation ≥ 3.
- **Slice:** add to `internal/drift/state.go`.
- **Context:** brief 007 (Forced Mode).
- **Depends-on:** `[phase-5]`
- **Done when:** crossing threshold returns true.

### Phase 7 — Correction generator

- **Outcome:** `drift.Correct(level, goal, action, history) (text, error)` via Haiku; level-specific templates; switches to forced-recovery template when `state.ForcedRecovery()`.
- **Slice:** `internal/drift/correct.go`; uses `prompts/drift_correct.txt`.
- **Context:** `internal/extract/client.go` (plan 011 phase 1); `internal/drift/state.go`.
- **Depends-on:** `[phase-3, phase-6]`
- **Done when:** fixture per level returns non-empty text containing goal.

### Phase 8 — Wire into proxy drift hook

- **Outcome:** Plan 002 phase 14's no-op consumer becomes: score → persist → level → (if `≥ nudge`) generate correction → splice into next request via plan 002 phase 9.
- **Slice:** `internal/drift/hook.go`.
- **Context:** `internal/proxy/drift_hook.go`; `internal/proxy/inject.go`; `internal/drift/score.go`, `level.go`, `persist.go`, `correct.go`.
- **Depends-on:** `[phase-2, phase-3, phase-4, phase-7]`
- **Done when:** drifting fixture → next request body contains the correction.

### Phase 9 — `kete drift-test` real impl

- **Outcome:** Replaces plan 001 phase 8 stub: prints `score=N level=X correction="..."`.
- **Slice:** rewrite `cmd/kete/drifttest.go`.
- **Context:** `cmd/kete/drifttest.go`; `internal/drift/score.go`, `level.go`, `correct.go`.
- **Depends-on:** `[phase-2, phase-3, phase-7]`
- **Done when:** end-to-end run on a fixture.

### Phase 10 — Surface escalation in `kete status`

- **Outcome:** `kete status --all` shows active sessions with `escalation=N`.
- **Slice:** `internal/drift/snapshot.go` + edits to `cmd/kete/status.go`.
- **Context:** `cmd/kete/status.go`; `internal/drift/state.go`.
- **Depends-on:** `[phase-5]`
- **Done when:** running proxy at escalation 2 → status shows it.

### Phase 11 — Doc: `docs/explanation/anti-drift.md`

- **Outcome:** Levels, thresholds, escalation, forced recovery, what's logged where.
- **Slice:** new file.
- **Context:** brief 007 Doc impact.
- **Depends-on:** `[phase-3]`
- **Done when:** file exists.

## Out of scope

- Mid-token interventions. Drift in MCP-only mode. A general policy engine.

## Assumptions

- Plan 011 lands first. Plan 002's drift hook + injection helpers are in place. Brief thresholds are correct.
