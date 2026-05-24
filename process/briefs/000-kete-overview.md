---
id: 000-kete-overview
date: 2026-05-24
status: shipped
from-idea: 2026-05-24-kete
design: null
adrs: [0000-project-identity-and-name, 0001-go-1-22-single-binary, 0002-sqlite-via-modernc-no-cgo, 0015-three-upstreams-selection]
plan: 000-kete-overview
---

# 000 — kete overview

## Problem

When dreamware-nz developers use Crush to drive an AI through a coding task, the model figures things out — discovers an architectural decision, traces an auth flow, identifies the right file to edit — and then the session ends and that reasoning is gone. The next session, on the same codebase, often the same task family, starts from zero. Two costs:

1. **Re-exploration.** A fresh Crush session pays the same exploration tax the previous one already paid. Tokens, time, attention.
2. **Lost decisions.** The "we chose X over Y because Z" rationale that emerged inside one session never reaches the next teammate's session, the next codebase, or even the same dev tomorrow.

There is also a within-session concern: the agent drifts (edits files outside the stated scope, repeats edits, takes tangents) and there's no scaffolding to catch it. Crush won't notice; the user will, ten minutes after the damage.

The shape of the solution is well-trodden — the existing `TonyStef/Grov` project does the same thing for Claude Code. We are not porting it. We are building a system that solves the same problem for *our* workflow on *our* tools, owing wire compatibility to no-one. ADR 0000 captures the framing.

## Who is hurt

- **dreamware-nz developers using Crush.** Today they pay the re-exploration tax on every session. There's no shared store of "what we figured out about this codebase".
- **The future second user** in a dreamware-nz team workflow. Without a captured-reasoning store, onboarding to a codebase is "ask the AI to figure it out from scratch, again, with your context budget".
- **The agent itself**, in a small way: when it drifts, there's no signal back. Captured drift summaries inform the next session's injection.

If we cannot name dreamware-nz developers as the user, the brief is not ready. We can.

## Constraints

- **Crush is the primary client.** Other Anthropic-shaped clients work because the wire surfaces are protocol-compliant, but they are not the design target.
- **Three upstreams supported simultaneously**: `anthropic-direct`, `cc-proxy`, `bedrock`. Routing per request via header → model-id pattern → env var. (ADR 0015.)
- **Go ≥ 1.22, single static binary**, no cgo. Pure-Go SQLite. (ADRs 0001, 0002.)
- **Local-first.** No cloud sync in v1. The captured-reasoning store at `~/.kete/memory.db` is the only durable state.
- **MCP-compliant stdio server** with two tools. Hand-rolled JSON-RPC (ADR 0012); no external SDK dependency until that ecosystem stabilises.
- **Apache-2.0** to match the dreamware-nz family.

## Success looks like

Three observable signals, in order:

1. **A Crush session pointed at kete completes a non-trivial task** (refactor, feature implementation, bug fix), and the captured `task` row in `~/.kete/memory.db` includes the goal, key decisions with rationale, files touched, and the reasoning trace. Verifiable via `kete tasks <project>`.
2. **A new Crush session against the same project**, started a day later, automatically receives the prior task's reasoning as injected context. The model's first response references the prior decision rather than re-investigating. Verifiable in the request log.
3. **Drift detection in a synthetic test**: `kete drift-test` over a fixture session that touches files outside the stated scope produces a drift score below 5 and a coherent correction message. Score doesn't have to match any external baseline; it has to be *useful*.

`kete status` cold-start under 50 ms is a soft target; nice to have, not a release gate.

## Non-goals

- **No team-sync v1.** Local store only. Sync is a future brief.
- **No multi-client design.** We don't ship Cursor/Zed/Antigravity capture pipelines speculatively. Crush is the user.
- **No wire-compat with TS-grov.** Different product. Different owner. We may share patterns; we share no bytes. (ADR 0000.)
- **No mid-token drift intervention.** Drift correction is between turns, not within a model response.
- **No mid-session model-swap handling.** dreamware-nz's normal workflow doesn't swap; when cc-proxy is in play it hides swaps anyway. We capture against the model the session ends on.
- **No dashboard.** CLI surface only.

## Open questions

- `[adr]` Already settled in the existing ADR set: Go version (0001), SQLite driver (0002), CLI framework (0010), HTTP server library (0005), adapter shape (0007), drift-level structure (0011), three-upstream routing (0018), Bedrock specifics (0016), MCP transport (0014).
- `[adr]` Replacement for ADR 0003 (idempotent ALTERs) — we now own the schema; clean numbered migrations are appropriate. ADR 0003 (pending).
- `[adr]` Replacement for ADR 0008 (MCP tool descriptions) — we own the tool descriptions; write them honestly rather than copy from TS. ADR 0008 (pending).
- `[adr]` 0016 — cloud-sync deferred. Settled.
- `[design]` 002-local-proxy is already drafted and stands.
- `[design]` Capture pipeline — single source (Crush via the proxy), so brief 006 collapses. Probably no separate design doc needed.

## Doc impact

- `README.md` `[new]` — quickstart for kete; the dreamware-nz lineage; pointer to cc-proxy.
- `docs/tutorials/first-run.md` `[new]`.
- `docs/explanation/why-proxy-not-just-mcp.md` `[new]` — the load-bearing argument for the proxy: tool-use is cooperative, hooks are cooperative, the proxy is not. (See brief 002.)
- `docs/explanation/three-upstreams.md` `[new]`.
- `docs/reference/cli.md`, `docs/reference/env.md`, `docs/reference/proxy.md`, `docs/reference/mcp.md`, `docs/reference/schema.md` `[new]`.
- `CHANGELOG.md` `[new]` — start at `0.1.0`.
