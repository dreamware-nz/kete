---
id: 007-anti-drift
date: 2026-05-24
status: accepted
from-idea: 2026-05-24-kete
design: null
adrs: [0011-drift-correction-four-levels]
plan: 007-anti-drift
---

# 007 — Anti-drift detection and correction

## Problem

Within a single session, a model agent goes off-task: starts editing files outside the user's stated scope, repeats edits to the same file (circling), or drifts into tangents. The TS implementation watches actions (Edit/Write/Bash from the model's tool calls), scores alignment 1–10 against the session's extracted intent (via Haiku), and injects a correction when the score drops. Four correction levels — `nudge` (score 7), `correct` (5–6), `intervene` (3–4), `halt` (1–2). Score ≥ 8 is "no action". Repeated drift escalates; recovery (score back ≥ 8) decrements the escalation count.

The mechanism only works if the proxy can see actions in real time and can mutate the response stream to inject corrections. That's why drift detection lives in the proxy (`drift-checker-proxy.ts`, `correction-builder-proxy.ts`) rather than in the legacy hook path. kete implements this same scoring + escalation loop, with the same level thresholds, against the same `steps` and `drift_log` tables.

## Who is hurt

- Users running Claude Code through the proxy on long, complex tasks. Without anti-drift the agent burns tokens on tangents; with it broken the captured `steps` are polluted with rejected actions and the user sees stale corrections.
- Anyone running `kete drift-test "<prompt>" --goal "<goal>"`. The command is the only debug surface for this subsystem.

## Constraints

- Same four correction levels at the same score thresholds. (`8–10 none`, `7 nudge`, `5–6 correct`, `3–4 intervene`, `1–2 halt`.)
- Same `steps` table: `(action_type, files, folders, command, reasoning, drift_score, drift_type, is_key_decision, is_validated, correction_given, correction_level, keywords, timestamp)`.
- Same `drift_log` table for actions with `drift_score < 5` (rejected — saved to log, not promoted to `steps`).
- Drift checks run every `DRIFT_CHECK_INTERVAL` prompts (default 5). Configurable via env.
- Escalation count increments on each correction; on score ≥ 8 it decrements. Forced-recovery mode kicks in at escalation ≥ 3 (per `plan_proxy_local.md` "Forced Mode").
- All scoring calls go through the same Haiku model the proxy uses for everything else (brief 011).
- Correction injection uses raw-body editing, same byte-exact discipline as memory injection.

## Success looks like

- `kete drift-test` on a fixture set produces drift scores consistent across runs (deterministic prompts; same fixture → same level), and the assigned correction level matches author judgement on a hand-labelled set of ≥ 20 fixture sessions.
- A live Claude Code session demonstrably halts (score 1–2) when the operator manually steers it off-task. The halt message contains the original goal and a recovery plan.
- Escalation increments and decrements are visible in the `drift_log` audit trail.

## Non-goals

- Adding a fifth correction level. Four is the contract.
- Changing the score thresholds. They're tuned against captured test sessions; new thresholds would need a new tuning pass and a new ADR.
- Drift detection outside the proxy (i.e. for IDE/MCP modes that don't proxy). The TS code doesn't do this either; without raw response stream access there's nowhere to inject a correction.
- A general policy engine. The drift checker is one rule set; resist generalising.

## Open questions

- `[adr]` 0011 — Ratify the four-level structure and thresholds. Cite `docs/antidrift.md` and the drift checker source.
- `[adr]` Extraction model: stay on `claude-haiku-4-5-20251001` or follow whatever the proxy is forwarding to. (Probably stay on Haiku — drift detection doesn't need to use the user's chosen model.)
- How to surface escalation state to `kete proxy-status`. Probably a column in the active-sessions table.

## Doc impact

- `docs/explanation/anti-drift.md` `[new]` — copy the level table from `Grov-Original/docs/antidrift.md` and pin it.
- `docs/how-to/test-drift.md` `[new]` — `kete drift-test` recipe.
- `docs/reference/env.md` `[update]` — `DRIFT_CHECK_INTERVAL`.
