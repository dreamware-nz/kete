---
id: 010-memory-injection
date: 2026-05-24
status: accepted
from-idea: 2026-05-24-kete
design: null
adrs: []
plan: 010-memory-injection
---

# 010 — Memory injection

## Problem

Captured tasks become useful when they're injected into a *new* session. The TS implementation has two injection paths:

1. **Pre-session (proxy mode).** When the proxy sees a fresh request, `preprocess.ts` queries the cloud (or local DB if sync is off) for memories matching the project + the user's prompt, and rewrites the request body to include them. Two sub-modes:
   - *Inline* — full memory text injected into the request's system prompt.
   - *Tool-loop* — a `kete_expand` tool is added to the request; the model picks an 8-char ID; the proxy intercepts the tool call, returns the full memory, and forwards the next turn. Up to 5 cycles per request.
2. **Per-prompt (MCP mode).** The IDE/CLI calls `kete_preview(context, mode)` on every prompt; the model decides which previewed memory (if any) is relevant and calls `kete_expand(id)`.

Both paths converge on `injection/memory-injection.ts` for body rewriting and `cache.ts` for the 8-char ID cache. kete implements both. The proxy path has the extra constraint that the injected text becomes part of the cached prefix — once a memory is injected, the byte-exact discipline applies to it forever.

## Who is hurt

- Users who have memories in the DB but don't see them used. Silent injection failures look like the product not working.
- Anyone whose prompt cache invalidates because injection isn't byte-stable across requests for the same conversation.

## Constraints

- Both injection paths share the same cache module (`cache.ts` in TS) keyed by 8-char ID. Cache lifetime: one preview→expand cycle in MCP mode; one request in proxy mode.
- Default ranking: the cloud's `fetchTeamMemories(teamId, projectPath, { context, limit })` decides relevance. Local fallback (no sync) ranks by `tasks.created_at DESC` filtered by `project_path` (simple but matches TS's local fallback).
- `kete_expand` tool loop hard cap: 5 cycles per request. Past that, return a synthesised "no further expansion" response and let the model continue.
- `addInjectionRecord` writes a row indicating which memories were injected when, for analytics (`reportInjection` in TS).
- Project-path key matching: must use the same folder-name normalisation that capture uses (brief 003, brief 006). A mismatch silently kills injection.
- For Anthropic requests, injected text must be inserted before the `cache_control` breakpoint to be eligible for caching itself.

## Success looks like

- A captured task in project X is visible in a new session in project X (same machine) within one prompt.
- A teammate on a different machine, with sync enabled, sees the same memory injected on a related prompt within five seconds of sync completing.
- The `kete_expand` tool loop terminates correctly at 5 cycles even on a model that keeps requesting new memories.
- A request that injects two memories has a stable byte-prefix across consecutive prompts in the same conversation (verifiable via cache hit metrics).

## Non-goals

- New ranking algorithms in v1. Whatever the cloud returns is what we use.
- A user-facing "show injection" surface. Today it's a log line; that's enough.
- Cross-project injection. A memory in project A does not surface in project B.
- Embedding-based local search. The cloud has it; the local fallback is timestamp-ordered.

## Open questions

- `[adr]` Whether the local fallback should do a simple FTS5 search (SQLite ships it) or stay timestamp-only. TS is timestamp-only; FTS5 is cheap, but adding it now is scope creep.
- How to handle the `kete_expand` tool loop on Codex (`/v1/responses`). The TS adapter has a parallel implementation; we have to mirror it without unifying prematurely.
- Reporting: how kete calls `reportInjection` without blocking the response path. (Spawn a goroutine; surface failures only in `kete doctor`.)

## Doc impact

- `docs/explanation/injection.md` `[new]` — the two injection paths, cache key, project-path normalisation.
- `docs/how-to/diagnose-no-injection.md` `[new]`.
- `docs/reference/proxy.md` `[update]` — note the `kete_expand` tool loop cap.
