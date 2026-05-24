---
number: 0006
title: Preserve raw request bodies as []byte; never re-serialise JSON in the proxy hot path
date: 2026-05-24
status: accepted
brief: 002-local-proxy
supersedes: null
superseded-by: null
---

# 0006 — Preserve raw request bodies as []byte; never re-serialise JSON in the proxy hot path

## Context

Anthropic's prompt cache matches requests by a byte-exact prefix of the request body. Two requests that decode to the same JSON object but serialise differently — different whitespace, different key order, different number formatting (`1.0` vs `1`), different escape style (`\u003c` vs `<`) — produce different ETags and miss the cache. A cache miss costs ~12.5× more than a hit (`cache_creation` is 1.25×, `cache_read` is 0.1×) and counts against the user's ITPM rate limit.

The TS implementation is byte-pedantic about this:

- `addContentTypeParser('application/json', { parseAs: 'buffer' })` — Fastify gets the raw bytes, parses for inspection, but `request.rawBody` holds the original.
- All injection (memory, drift correction, extended-cache keep-alive) is implemented as raw-string slicing on `rawBody`. There are explicit `// CRITICAL` comments around `max_tokens` and `stream` (which appear before the cache breakpoint).
- The forwarding helper sends `rawBody` as the upstream request body, not a re-serialised JSON.

`Grov-Original/AGENTS.md` calls this out as a top-level gotcha.

kete does the same. Go's `encoding/json` is even more dangerous here than `JSON.stringify`: it sorts map keys alphabetically. A request that came in with `{"messages": [...], "model": "..."}` and was unmarshaled to a `map[string]any` then re-marshaled would come out `{"messages": [...], "model": "..."}` (alphabetised — fine on this example, broken on others). And once we add a struct definition it's worse: struct field order is the *struct definition* order, not the input order.

## Decision

The proxy treats the request body as `[]byte` and passes the same `[]byte` upstream. We never call `json.Marshal` on a parsed body in the forward path.

For inspection, we `json.Unmarshal` into a typed view (struct or `map[string]any`), use it read-only, and discard. For mutation (memory injection, drift correction, keep-alive), we operate on the `[]byte` with byte-offset edits, then validate the result still parses (and structurally matches our expectation) before forwarding.

Concretely, mutation is implemented as a small set of helpers on a `RawBody` type:

- `RawBody.InsertBeforeMessagesClose(payload []byte) error` — find the closing `]` of the `messages` array, splice `,<payload>` before it.
- `RawBody.InsertIntoSystemPrompt(text string) error` — find the system field's string value, splice text into it without re-quoting.
- `RawBody.AddTool(tool []byte) error` — find the closing `]` of the `tools` array (or create the array if absent), splice in the tool definition.

Each helper validates the post-mutation buffer parses as JSON and that the cache-relevant prefix (everything up to the first `cache_control` breakpoint) is still byte-identical to where it should be.

We do NOT touch `max_tokens` or `stream` in any helper — they appear before user-controlled content and altering them would change the prefix.

## Options considered

- **Raw-byte mutation with typed inspection.** What we picked. Mirrors the TS discipline, gains type safety on the inspection side.
- **Unmarshal into a struct, marshal on the way out.** Type-safe but defeats the cache. Reject. (We can write a unit test that demonstrates it fails: same input, same struct round-trip, different bytes out, different cache hit rate. We won't add the test now, but we will when something tries to "simplify" this.)
- **A "JSON pointer + edit" library.** Overkill. The mutations are three or four specific patterns; a tiny purpose-built helper is honest.

## Consequences

Easier:

- Cache hit rates match the TS implementation by construction.
- The injection code is small and obvious.
- We can extend the helpers as new injection points show up, with no risk of "accidentally re-serialising".

Harder:

- We must resist the obvious refactor temptation. New contributors will see `[]byte` slicing and reach for `json.Marshal`. This ADR exists so a code review can point at it.
- Bugs in the byte-offset finder (e.g. naive bracket-counting that ignores escaped strings) are real. The helpers must treat strings as opaque (skip over `"…"` regions including escaped quotes) when scanning for structural characters. Cover with fuzz tests.
- Streaming responses are handled separately (no cache implication on the response side), but parsing them still uses byte-offset SSE chunk handling for symmetry.
