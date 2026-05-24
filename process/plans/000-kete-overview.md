---
id: 000-kete-overview
date: 2026-05-24
status: done
brief: 000-kete-overview
design: null
adrs: [0000-project-identity-and-name, 0001-go-1-22-single-binary, 0002-sqlite-via-modernc-no-cgo, 0015-three-upstreams-selection]
---

# 000 — kete overview (master plan)

## Goal

A working `kete` v1: capture, inject, drift-correct, and auto-compact a Crush session against any of three upstreams, persisted in `~/.kete/memory.db`, shipped as a single static binary.

This plan sequences the per-component plans. Each phase here is "land plan NNN to status: done"; the microphases live in those plans.

## Phases

### Phase 1 — Store ready

- **Outcome:** Plan `004-memory-store` complete; DB has `tasks`, `steps`, `drift_log`, `sync_tracker`.
- **Slice:** execute plan 004.
- **Context:** `process/plans/004-memory-store.md`.
- **Depends-on:** `[]`
- **Done when:** plan 004 status `done`.
- **Status:** done — plan 004 complete prior to this run.

### Phase 2 — CLI shell ready

- **Outcome:** Plan `001-cli-shell` complete; subcommands work, stubs in place for proxy/mcp/drift-test.
- **Slice:** execute plan 001.
- **Context:** `process/plans/001-cli-shell.md`.
- **Depends-on:** `[phase-1]`
- **Done when:** plan 001 status `done`.
- **Status:** done 2026-05-24 — 13 phases landed; `kete` binary builds, `make docs` regenerates `docs/reference/cli.md`.

### Phase 3 — Extraction client ready

- **Outcome:** Plan `011-reasoning-extraction` phases 1–6 complete (wiring 7–9 unblock later).
- **Slice:** execute plan 011 phases 1–6.
- **Context:** `process/plans/011-reasoning-extraction.md`.
- **Depends-on:** `[phase-1]`
- **Done when:** Haiku client + prompts + retry + cost discipline in place.

### Phase 4 — Local proxy ready (anthropic-direct)

- **Outcome:** Plan `002-local-proxy` complete; capture writes raw rows; injection helpers + drift/compaction hooks (no-op).
- **Slice:** execute plan 002.
- **Context:** `process/plans/002-local-proxy.md`.
- **Depends-on:** `[phase-1, phase-2]`
- **Done when:** plan 002 status `done`.

### Phase 5 — MCP server ready

- **Outcome:** Plan `003-mcp-server` complete.
- **Slice:** execute plan 003.
- **Context:** `process/plans/003-mcp-server.md`.
- **Depends-on:** `[phase-1, phase-2]`
- **Done when:** plan 003 status `done`.
- **Status:** done 2026-05-24 — `kete mcp` ships `initialize`, `tools/list`, `tools/call` for `kete_preview` and `kete_expand`; e2e verified.

### Phase 6 — Capture pipelines ready

- **Outcome:** Plan `006-capture-pipelines` complete; all five sources land tasks; `sync_tracker` dedupes.
- **Slice:** execute plan 006; wires plan 011 phase 7.
- **Context:** `process/plans/006-capture-pipelines.md`; `process/plans/011-reasoning-extraction.md`.
- **Depends-on:** `[phase-3, phase-4, phase-5]`
- **Done when:** plan 006 status `done`.

### Phase 7 — Memory injection ready

- **Outcome:** Plan `010-memory-injection` complete; proxy injects relevant prior tasks; MCP `kete_expand` resolves.
- **Slice:** execute plan 010.
- **Context:** `process/plans/010-memory-injection.md`.
- **Depends-on:** `[phase-4, phase-5, phase-6]`
- **Done when:** brief 000 success signal #2 verified.

### Phase 8 — Anti-drift ready

- **Outcome:** Plan `007-anti-drift` complete; wires plan 011 phase 8.
- **Slice:** execute plan 007.
- **Context:** `process/plans/007-anti-drift.md`.
- **Depends-on:** `[phase-3, phase-4]`
- **Done when:** plan 007 status `done`.

### Phase 9 — Auto-compaction ready

- **Outcome:** Plan `008-auto-compaction` complete; wires plan 011 phase 9.
- **Slice:** execute plan 008.
- **Context:** `process/plans/008-auto-compaction.md`.
- **Depends-on:** `[phase-3, phase-4]`
- **Done when:** plan 008 status `done`.

### Phase 10 — Extended cache ready

- **Outcome:** Plan `009-extended-cache` complete.
- **Slice:** execute plan 009.
- **Context:** `process/plans/009-extended-cache.md`.
- **Depends-on:** `[phase-4]`
- **Done when:** plan 009 status `done`.

### Phase 11 — Bedrock upstream ready

- **Outcome:** Plan `012-bedrock-vendor` complete.
- **Slice:** execute plan 012.
- **Context:** `process/plans/012-bedrock-vendor.md`.
- **Depends-on:** `[phase-4]`
- **Done when:** plan 012 status `done`.

### Phase 12 — cc-proxy upstream ready

- **Outcome:** Plan `013-cc-proxy-upstream` complete.
- **Slice:** execute plan 013.
- **Context:** `process/plans/013-cc-proxy-upstream.md`; `process/plans/012-bedrock-vendor.md` (selector landed there).
- **Depends-on:** `[phase-4, phase-11]`
- **Done when:** plan 013 status `done`.

### Phase 13 — Release polish

- **Outcome:** README, CHANGELOG, top-level docs final; tagged `0.1.0`.
- **Slice:** finalise README quickstart for all upstreams; CHANGELOG bootstrapped; ldflags-driven version.
- **Context:** brief 000 Doc impact; all per-plan doc phases.
- **Depends-on:** `[phase-7, phase-8, phase-9, phase-10, phase-12]`
- **Done when:** `kete --version` prints `0.1.0`; `make release` produces darwin/linux binaries.

## Out of scope

- Cloud sync (plan 005, deferred). Multi-client beyond plan 006's five. A dashboard. OpenAI `/v1/responses`. Mid-token drift.

## Assumptions

- Plans in different branches of the DAG can develop in parallel; whoever lands the shared selector code (plan 002 phase 8) first, others rebase.
- Within each plan, microphases are executable as written; a blocked plan stops the master at that phase.
