---
number: 0008
title: Two MCP tools (kete_preview, kete_expand) with kete-authored descriptions
date: 2026-05-24
status: accepted
brief: 003-mcp-server
supersedes: null
superseded-by: null
---

# 0008 — Two MCP tools (kete_preview, kete_expand) with kete-authored descriptions

## Context

The MCP surface kete exposes is two tools: `kete_preview` and `kete_expand`. This is a deliberate two-step shape — preview a candidate set, expand one by ID — that addresses a cost trade-off honestly. Returning full memories on every preview wastes tokens. Returning only IDs forces the model to commit blindly. Preview-with-summaries gives the model enough information to commit to one memory, then expand fetches the full reasoning. The shape is converged-on, not novel: the same family of trade-offs led grov-the-product to the same surface.

The tool descriptions are part of the wire surface — they're literally what the model reads to decide whether and how to call the tools. They are tuned content, not throwaway prose. This ADR pins both the shape and the ownership of that content.

## Decision

The kete MCP server exposes exactly two tools:

- `kete_preview(context: string, mode: string)` — returns up to 3 memory previews matching the context string, each with an 8-character ID and a short summary.
- `kete_expand(id: string)` — returns the full reasoning trace for a previewed memory by its 8-character ID.

The tool descriptions are **kete-authored**, not borrowed. They live in `internal/mcp/tools/preview.txt` and `internal/mcp/tools/expand.txt`, embedded via `go:embed`. They are tuned for the models Crush typically uses (Sonnet 4-class as primary; Haiku 4-class for cheap subagents).

The descriptions are part of the wire surface — changing them affects model behaviour. Edits to either file are an ADR-level decision; commits that touch them require a brief justification in the commit message and a passing test that exercises the affected tool's call shape.

We keep strong language in `kete_preview`'s description asking the model to call it on every prompt, because the cost analysis demands it: a missed preview means a missed retrieval, and there is no way to recover the lost context.

## Options considered

- **Two tools, kete-authored descriptions.** What we picked.
- **One combined tool returning full bodies.** Wastes tokens on every preview. Reject — same reasoning the two-tool shape is converged on.
- **Three tools (add a "feedback this memory was useful" tool).** Additional surface, no demonstrated win. The proxy can infer usefulness from injection records (when we add that). Reject for v1.

## Consequences

Easier:

- The descriptions can iterate against real Crush + Haiku/Sonnet behaviour without external pressure to mimic someone else's wording.
- Ownership is obvious: kete's tool descriptions live in kete's repo and are maintained by kete's authors.
- The test surface is well-bounded: we test our own description against our own expectation.

Harder:

- Writing the descriptions well takes work. We will have v1 descriptions that aren't perfect, and we'll iterate. The first version of `preview.txt` and `expand.txt` is *good enough to ship*, not *final*.
- Without an eval harness, "is the new description better" is judgement. We do not invent an eval framework preemptively; we trust author judgement on the small text edits that arise.

If a future eval setup shows a clearly better description, that's a normal commit (small text change), not a new ADR. The ADR-level concern is the *two-tool shape* and *we own the strings* — not the wording itself.
