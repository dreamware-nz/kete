---
id: 009-extended-cache
date: 2026-05-24
status: draft
brief: 009-extended-cache
design: null
adrs: [0013-byte-exact-keepalive-injection]
---

# 009 — Extended cache (keep-alive)

## Goal

An opt-in keep-alive timer that re-sends the most recent request body byte-identically (with a single appended `,{"role":"user","content":"."}`) to keep Anthropic's prompt cache warm.

## Phases

### Phase 1 — Flag + env

- **Outcome:** `kete proxy --extended-cache` and `KETE_EXTENDED_CACHE=true` enable; default off.
- **Slice:** edits to `cmd/kete/proxy.go` + `internal/proxy/server.go`.
- **Context:** `cmd/kete/proxy.go` (plan 001 phase 4); brief 009 Constraints.
- **Depends-on:** `[]`
- **Done when:** flag reaches the server with the right bool.

### Phase 2 — Per-session in-memory cache

- **Outcome:** Map keyed by session id; entries `{headers, rawBody, lastSeen, keepAliveCount}`; eviction at 10 min.
- **Slice:** `internal/keepalive/store.go`.
- **Context:** brief 009 Constraints.
- **Depends-on:** `[]`
- **Done when:** unit test: store, retrieve, evict at 10 min.

### Phase 3 — Capture last request

- **Outcome:** On every successful forward (anthropic-direct + cc-proxy only), update the entry. Bedrock excluded.
- **Slice:** `internal/keepalive/capture.go` hooked into `internal/proxy/forward.go`.
- **Context:** `internal/proxy/forward.go`; `internal/keepalive/store.go`.
- **Depends-on:** `[phase-2]`
- **Done when:** integration test: 1 request → 1 entry.

### Phase 4 — Byte-exact splice helper

- **Outcome:** `keepalive.AppendDot(rawBody) ([]byte, error)` splices `,{"role":"user","content":"."}` immediately before the closing `]` of `messages`.
- **Slice:** `internal/keepalive/splice.go`.
- **Context:** ADR 0013; `internal/proxy/inject.go` (reusable scanner).
- **Depends-on:** `[]`
- **Done when:** test asserts byte-for-byte difference is exactly the segment.

### Phase 5 — Idle timer

- **Outcome:** 60 s tick; sends keep-alive when `4 min ≤ idle ≤ 10 min` and `count < 2`.
- **Slice:** `internal/keepalive/timer.go`.
- **Context:** `internal/keepalive/store.go`; brief 009 Constraints.
- **Depends-on:** `[phase-2]`
- **Done when:** synthetic-clock test: 4 min idle triggers exactly one send.

### Phase 6 — Send + discard

- **Outcome:** Keep-alive POSTs through the existing adapter; response read and discarded; errors logged.
- **Slice:** `internal/keepalive/send.go`.
- **Context:** `internal/adapter/anthropic.go`; `internal/keepalive/splice.go`, `timer.go`.
- **Depends-on:** `[phase-4, phase-5]`
- **Done when:** integration test: stub upstream sees a body ending with the dot segment.

### Phase 7 — Cleanup on shutdown

- **Outcome:** SIGINT/SIGTERM stops the timer; no goroutine leak.
- **Slice:** signal wiring in `internal/keepalive/timer.go`.
- **Context:** `internal/proxy/server.go` (graceful shutdown).
- **Depends-on:** `[phase-5]`
- **Done when:** `goleak` clean after shutdown.

### Phase 8 — Doc: `docs/how-to/extended-cache.md`

- **Outcome:** What it does, consent posture, constants, when not to use it.
- **Slice:** new file.
- **Context:** brief 009 Doc impact.
- **Depends-on:** `[]`
- **Done when:** file exists.

## Out of scope

- Keep-alive for OpenAI/Codex. Cross-process state. Tunable timing. Bedrock keep-alive.

## Assumptions

- Plan 002 phase 9's scanner is reusable. Brief constants are right. Anthropic doesn't change the `messages` shape.
