---
id: 006-capture-pipelines
date: 2026-05-24
status: shipped
from-idea: 2026-05-24-kete
design: null
adrs: [0014-jsonl-session-poll-vs-hooks]
plan: 006-capture-pipelines
---

# 006 — Capture pipelines

## Problem

Reasoning gets into the local DB through five distinct mechanisms, depending on how the user is talking to a model:

1. **Proxy capture** (Claude Code, Codex CLI through the local proxy). The orchestrator buffers the model response, extracts reasoning on `end_turn`, and writes a `tasks` row. This is the canonical path; everything else is a workaround.
2. **MCP IDE capture** (Cursor, Zed, Antigravity in IDE mode). The IDE doesn't go through the proxy; the MCP server is read-only. Capture happens via IDE-specific hooks (`hook-handler.ts`) that the IDE runs on session events.
3. **MCP CLI capture** (Cursor CLI, Codex CLI when not proxied). Polling-driven: a watcher (`cli-watcher.ts`) tails `~/.cursor/chats` (or equivalent), an extractor (`cli-extractor.ts`) parses chat sessions into the same `tasks` shape.
4. **Antigravity-specific scanner** (`antigravity-scanner.ts`, `antigravity-parser.ts`, `antigravity-sync-tracker.ts`). Antigravity stores chats in a SQLite-like format under its app dir; we scan it on a timer, parse, and dedupe via a `sync-tracker` so we don't double-capture.
5. **Hooks** (legacy Claude Code `SessionStart`/`Stop` hooks). Removed in `SESSION_DEC2_2025_HOOKS_REMOVAL.md`. Mentioned here for the record; kete does not implement hook capture.

All five paths converge on the same `tasks` table with the same row shape. The convergence rules (when to dedupe, when to update vs insert, how to detect "session ended") are scattered across the TS code and need to be lifted into a single pipeline contract.

## Who is hurt

- Users who switch tools mid-task (start in Claude Code, finish in Cursor). Today the captured reasoning is split across two tasks; the convergence story is fragile.
- Users on Antigravity, where capture is filesystem-scanning and any change to Antigravity's storage layout breaks us.
- Anyone using a tool we don't have explicit support for — silent, captures nothing, no error.

## Constraints

- Five capture sources stay (modulo hooks, which are removed). Adding a sixth is out of scope.
- All sources write through the same store API (`createTask`, `updateTask`); no source has a direct path to SQLite.
- A `sync_tracker` table (or equivalent local state) prevents double-capture. The TS implementation uses different trackers per source; kete should unify on one shape.
- Reasoning extraction (LLM-driven; brief 011) runs after capture, not inline. Capture writes a raw row; an extractor pass enriches it.
- Capture must not block the proxy's request path. If the model returns and capture is slow, the user sees the response immediately and capture finishes async.

## Success looks like

- A Claude Code session through the Go proxy, then a Cursor IDE session against the same project, both produce `tasks` rows with consistent `project_path` (the folder-name normalisation from brief 003) and the same `user_id`.
- Antigravity scan picks up a new chat within 60 s of the user finishing it, no duplicate row on subsequent scans.
- A capture failure (parse error, permission denied) emits a structured log line and does not abort the run.

## Non-goals

- New capture sources. VS Code (TS roadmap) and Gemini CLI (TS roadmap) land when they land upstream.
- Real-time capture for IDE-mode integrations that don't expose hooks. Polling is good enough.
- Reverse-engineering Antigravity's file format if upstream changes it. We pin to the version that works at port time and bump deliberately.

## Open questions

- `[adr]` 0014 — JSONL polling vs hooks: re-affirm that hooks are gone and polling/MCP is the path forward. Cite `SESSION_DEC2_2025_HOOKS_REMOVAL.md`.
- `[adr]` Unified `sync_tracker` shape (or per-source trackers). One table is cleaner; per-source might be inevitable because the dedup keys differ.
- `[adr]` Where the Antigravity scanner runs: only inside the MCP server process (TS choice) or as an independent goroutine in `kete proxy` too. TS choice makes sense — only one process is "always running".
- `[design]` Capture pipeline as a whole — when this brief moves into the plan stage, a design doc covering the convergence rules across all sources is probably warranted.

## Doc impact

- `docs/explanation/capture.md` `[new]` — the five paths and how they converge.
- `docs/reference/supported-tools.md` `[new]` — table mirroring the TS README's "Supported Tools" table.
- `docs/how-to/diagnose-missing-capture.md` `[new]`.
