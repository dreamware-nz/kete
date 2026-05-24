---
project: kete
date: 2026-05-24
status: ready
---

# kete

> A local memory and reasoning layer for AI coding sessions, sized for the dreamware-nz workflow on Crush. Not a port of grov; a new system that shares one of grov's problems.
>
> Name from te reo Māori — `kete` is a woven basket; `ngā kete o te wānanga` ("the three baskets of knowledge") is the canonical Māori metaphor for collected, portable knowledge. See ADR 0000.

## One line

A local-first proxy + MCP server that captures reasoning from Crush sessions, persists it in a SQLite store, and injects relevant prior reasoning into new sessions.

## What it does

Sits between Crush (the only first-class client) and whichever upstream Crush has chosen — Anthropic-direct, `dreamware-nz/cc-proxy` (subscription-backed), or AWS Bedrock — and uses the round trip to do four things the model and the IDE alone can't:

1. **Capture** — when a turn ends, extract the goal, key decisions, files touched, and reasoning trace; persist as an immutable `task` row.
2. **Inject** — when a new prompt starts, find relevant prior tasks for this project and add them to the request before the model sees it.
3. **Detect drift** — score the agent's actions against the session's stated goal; at low scores, inject a correction inline so the agent steers back.
4. **Auto-compact** — when the context window fills, replace the conversation with a structured summary that preserves the original goal, decisions, constraints, and current state.

Capture and injection also run via a stdio MCP server that exposes two tools (`kete_preview`, `kete_expand`) for *belt-and-braces*: when the model is smart enough to call tools reliably, the MCP path complements the proxy; when it isn't, the proxy carries the load alone. Neither layer depends on the other.

There is no team-sync in v1. The store is local. A team backend is a future brief, designed and built when there's a second user.

## Domain

Local-first developer tooling at the boundary between Crush and a model API. Specifically:

- **HTTP proxying with byte-exact request passthrough.** Anthropic's prompt cache is matched by a prefix ETag; re-serialising parsed JSON kills the cache.
- **MCP (Model Context Protocol)** server over stdio.
- **Embedded SQLite** as the durable memory store.
- **LLM-based extraction** — a Haiku-class model summarises reasoning, scores drift, and builds correction text.
- **Streaming protocol mediation** — Anthropic SSE in the request path, including mid-stream injection points; AWS Bedrock event-stream demuxed back to SSE for the client.
- **AWS SigV4 request signing** when the upstream is Bedrock.

## Primary users

- **dreamware-nz developers using Crush** as their primary AI coding tool. That is the user. Other Anthropic-shaped clients (Claude Code, Cursor, Zed CLI, Codex CLI) work because the proxy and MCP surfaces are protocol-compliant, but they are not designed for.
- **`dreamware-nz/cc-proxy`** as the upstream for subscription-backed Anthropic traffic. Compositional, not a dependency.

## Non-negotiables

- **Go ≥ 1.22.** Single static binary per OS/arch (`darwin/{amd64,arm64}`, `linux/{amd64,arm64}`). Pure-Go SQLite (`modernc.org/sqlite`); no cgo.
- **Three upstreams supported simultaneously**: `anthropic` | `cc-proxy` | `bedrock`. Selection per request via header → model-id pattern → env var. (ADR 0015.)
- **Byte-exact discipline on the request path.** ADR 0006. The proxy never re-serialises JSON for forwarding; injection edits raw bytes in place. Bedrock is the deliberate exception (ADR 0014).
- **Local DB at `~/.kete/memory.db`**, mode `0600`, parent dir `0700`. We own the schema. (ADR 0000.)
- **MCP stdio JSON-RPC compliant.** Two tools: `kete_preview`, `kete_expand`. Hand-rolled (ADR 0012).
- **Apache-2.0** to match cc-proxy and the broader dreamware-nz family.
- **No telemetry.** Local until a deliberate sync brief lands.

## Out of scope (for this project)

- A team-sync backend. Local-first; sync is a future brief once a second user exists.
- Multi-client design. Crush is the user. If a second client wants in, it implements our MCP and HTTP-proxy contracts.
- Wire compatibility with TS-grov (`TonyStef/Grov`). Different product, different owner. ADR 0000.
- A dashboard. The CLI is the surface for v1.
- Real-time mid-turn drift correction beyond what the proxy can inject between turns. Async correction is the bar; per-token intervention is not.
- Mid-session model-swap handling. dreamware-nz's normal workflow doesn't swap, and when cc-proxy is in play it abstracts swaps away. If a model swaps mid-session, we capture against whichever model the session ended on.

## Documentation

- **Audience(s):** dreamware-nz devs (end users), agents working in this repo, future contributors.
- **Shape (Diátaxis):**
  - *Tutorial* — `docs/tutorials/first-run.md` — install, point Crush at it, see a captured task.
  - *How-to* — `docs/how-to/use-cc-proxy.md`, `docs/how-to/use-bedrock.md`, `docs/how-to/inspect-memory.md`.
  - *Reference* — `docs/reference/cli.md`, `docs/reference/env.md`, `docs/reference/proxy.md`, `docs/reference/mcp.md`, `docs/reference/schema.md`.
  - *Explanation* — `docs/explanation/why-proxy-not-just-mcp.md`, `docs/explanation/raw-body-preservation.md`, `docs/explanation/three-upstreams.md`.
- **Home:** `docs/` in this repo. Generated where possible (CLI help → `cli.md`, schema → `schema.md`).

## Notes

- Reference implementation for inspiration only: `~/Documents/johnjansen/grov/Grov-Original/` (sibling clone of `TonyStef/Grov`). Read it for ideas, copy patterns where they earn their keep, owe it nothing on the wire.
- Upstream sibling project: `dreamware-nz/cc-proxy` — the subscription-backed Anthropic relay. Stacks under kete. Brief 013 covers the integration; cc-proxy's repo has its own ADR set worth reading for the OAuth/billing-prefix patterns.
- The `process/` chain was backfilled before any code was written. ADR 0000 is the project-identity frame; everything else hangs off it.
