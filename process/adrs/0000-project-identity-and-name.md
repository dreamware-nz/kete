---
number: 0000
title: Project identity — name "kete", scope, and what we are not
date: 2026-05-24
status: accepted
brief: 000-kete-overview
supersedes: null
superseded-by: null
---

# 0000 — Project identity: name "kete", scope, and what we are not

## Context

This ADR establishes the project's identity before any other decision is made. Three things are settled here so the rest of the corpus has a stable frame:

1. **The project's name.**
2. **Who the user is.**
3. **What relationship, if any, this project has to other systems that solve adjacent problems.**

## Decision

**Name.** The project is **kete**. The binary is `kete`. The repository is `dreamware-nz/kete`.

The name is from te reo Māori — a kete is a woven basket, and `ngā kete o te wānanga` ("the three baskets of knowledge") is the canonical metaphor in Māori epistemology for collected knowledge made portable. The metaphor matches what the system does: collect reasoning into a container, carry it across sessions, share it with the team. The name is short (four letters), greppable, cheap on a CLI, and culturally honest for a dreamware-nz project.

**User.** The user is **dreamware-nz developers using Crush**. Other Anthropic-shaped clients (Claude Code, Cursor, Zed CLI, Codex CLI) work because the proxy and MCP surfaces are protocol-compliant, but they are not the design target. We do not invest in compatibility with them speculatively.

**Scope.** kete is:

- A local HTTP proxy on `127.0.0.1:8080` that intercepts model API traffic for capture, injection, drift detection/correction, auto-compaction, and extended cache.
- A stdio MCP server exposing two tools (`kete_preview`, `kete_expand`).
- A CLI for setup, status, search, and operational tasks.
- A local SQLite store of captured reasoning at `~/.kete/memory.db`.

kete supports three upstreams: `anthropic-direct`, `cc-proxy` (`dreamware-nz/cc-proxy`), and `bedrock`. Selection per-request via header → model-id pattern → env var.

**What we are not.**

- *Not a continuation of `TonyStef/Grov` (the npm `grov` package).* That is a different product, owned by different people, deployed against a different cloud backend (`api.grov.dev`). We share a problem space; we share no bytes. Captured `tasks` from kete will not interoperate with grov, will not sync to `api.grov.dev`, and will not be readable by grov clients without an explicit, future, opt-in importer.
- *Not multi-client v1.* Crush is the user. If a second client wants in, it implements our MCP and HTTP-proxy contracts.
- *Not committed to a public release.* kete is a tool dreamware-nz uses internally. If it ever ships externally, that's a deliberate decision made later, with whatever licensing, support, and stability commitments come with that — none of which apply now.
- *Not bundled with cc-proxy.* `dreamware-nz/cc-proxy` is a peer; kete optionally forwards through it as one of the three upstreams. Separate repos, separate lifecycles.

## Options considered

- **kete.** What we picked. Te reo Māori; four letters; greppable; culturally apt; no major collisions in the developer-tooling space.
- **scribe.** Direct, English. Heavy collision risk (npm, PyPI, GitHub repos, internal tools at many companies). Reject.
- **mneme.** Crisp, Greek for memory. Spelling-confusable with "meme". Reject.
- **A working title that frames the project as a port** ("go-grov", "grov-go"). Reject — every artefact written under that title carries the framing of "port", and that framing is wrong.
- **No name yet, decide later.** Defers the cost of the decision into every artefact written before it. Reject.

## Consequences

Easier:

- Naming clarity. When discussing this project: "kete" means this; "grov" means TonyStef's product; "cc-proxy" means dreamware-nz/cc-proxy.
- Scope clarity. Every constraint we keep, we keep deliberately for kete's own reasons — not because some other product made the same choice for theirs.
- The CLI's surface, env var prefix (`KETE_*`), HTTP header namespace (`x-kete-*`), and on-disk path (`~/.kete/`) are all consistent.

Harder:

- We own every choice. There is no "fall back to what grov does" escape hatch when a design call is hard. That's the right cost for the freedom.
- A future case for interop (e.g. a grov user wants to migrate to kete) is a one-off importer brief, not a baked-in commitment. We will write that brief if and when a real user asks.

## Notes

The "three baskets" naming has a nice secondary echo for kete's architecture: the proxy holds *bytes* in flight, the store holds *facts* at rest, the MCP server holds *answers* on demand. Three vessels, three responsibilities. We do not lean on this in code or comments; it's just a frame that happens to fit.
