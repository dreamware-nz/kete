---
id: 009-extended-cache
date: 2026-05-24
status: accepted
from-idea: 2026-05-24-kete
design: null
adrs: [0013-byte-exact-keepalive-injection]
plan: 009-extended-cache
---

# 009 — Extended cache (keep prompt cache warm)

## Problem

Anthropic's prompt cache has a 5-minute TTL. When a developer pauses to read output, the next prompt often misses the cache and pays full `cache_creation` (1.25× base price, counts toward ITPM). The TS proxy ships an opt-in feature (`--extended-cache` / `KETE_EXTENDED_CACHE=true`) that sends a minimal "keep-alive" request (`,{"role":"user","content":"."}` appended to the messages array) every ~60 s during idle, refreshing the cache with the exact same prefix. Constants: idle threshold 4 minutes, max 2 keep-alives per idle period, total max idle ~10 minutes before cleanup.

The mechanism is wholly mechanical — but the byte-exact prefix discipline is what makes it work and what makes it dangerous. `JSON.stringify` would change the prefix and miss the cache; the TS code does raw-string slicing on the `rawBody` buffer with explicit warnings about not touching `max_tokens` or `stream`. kete does the same edit-by-byte-offset.

## Who is hurt

- Users on long-thought sessions where idle gaps are normal (architecture decisions, code review). Without extended cache they pay cache_creation on every "back from a meeting" prompt.
- The proxy itself, if the keep-alive logic is wrong: a single byte-mismatch turns every "save money" into "actively waste it on doomed cache_creation calls".

## Constraints

- Opt-in only. Default off. Enabled by `--extended-cache` flag or `KETE_EXTENDED_CACHE=true`. The TS README is explicit ("By using --extended-cache, you consent to Grov making API requests on your behalf.") — match that consent posture.
- Keep-alive request body is byte-identical to the most recent forwarded request, plus a single appended user message `{"role":"user","content":"."}` inserted before the closing `]` of the `messages` array. **Do not** modify `max_tokens` or `stream`; their position before the `messages` array means changing them invalidates the prefix.
- Timing: 60 s timer, 4 min idle threshold, 2 keep-alives per idle period, 10 min total max idle.
- Cost surface: ~$0.002 per keep-alive (per TS doc). Does not pollute the conversation; the response is discarded.
- Cleanup on `SIGINT` / `SIGTERM`; no orphan keep-alives if the proxy crashes (idempotent — Anthropic just times out the cache).
- In-memory cache map keyed by `session_id` (or equivalent), entries: `{ headers, rawBody, timestamp, keepAliveCount }`. Eviction policy: drop entries past 10-min total max idle.

## Success looks like

- Manual test: start a session, get a response, wait 6 minutes (past the 5-min TTL), send another prompt. With extended cache off: cache miss. With extended cache on: cache hit.
- The keep-alive request body, when diffed against the original forwarded body, differs by exactly the inserted `,{"role":"user","content":"."}` segment and nothing else (no whitespace differences, no key reordering, no number reformatting).
- A `kete proxy --extended-cache` run that survives an hour of idle without leaking goroutines.

## Non-goals

- Extending OpenAI's prompt cache. Different mechanism, different brief if the need shows up.
- Tuning the timing for individual users. The defaults are the contract.
- Caching across `kete proxy` restarts. In-memory only.

## Open questions

- `[adr]` 0015 — Byte-exact keep-alive injection: ratify the raw-string slicing pattern (find last `]` of `messages` array, splice in the extra entry, validate the result still parses as JSON before sending). Pin this in an ADR with the cited TS code.
- How aggressive to be about the 4-min threshold. TS doc notes a "TEMP: 1 minute for testing" comment — the production constants are 4 minutes idle / 2 keep-alives / 10 minutes total.

## Doc impact

- `docs/how-to/enable-extended-cache.md` `[new]`.
- `docs/explanation/extended-cache-cost-model.md` `[new]` — the math.
- `docs/reference/env.md` `[update]` — `KETE_EXTENDED_CACHE`.
