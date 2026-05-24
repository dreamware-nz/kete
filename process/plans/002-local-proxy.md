---
id: 002-local-proxy
date: 2026-05-24
status: active
brief: 002-local-proxy
design: 002-local-proxy
adrs: [0005-http-server-net-http-chi, 0006-raw-body-passthrough-for-prompt-cache, 0007-agent-agnostic-adapter-interface, 0009-haiku-as-extraction-model, 0011-drift-correction-four-levels, 0013-byte-exact-keepalive-injection, 0015-three-upstreams-selection]
---

# 002 — Local proxy

## Goal

A local HTTP proxy on `127.0.0.1:8080` that forwards `POST /v1/messages` byte-exact to the configured upstream, with capture/inject/drift/compaction layered as composable middlewares — none of which ever break the prompt-cache prefix.

## Phases

### Phase 1 — chi router + `/health`

- **Outcome:** `kete proxy` binds `127.0.0.1:8080`, returns `200 OK` on `GET /health`, 404 elsewhere.
- **Slice:** `internal/proxy/server.go`; chi router; `KETE_HOST`/`KETE_PORT` env.
- **Context:** ADR 0005; `cmd/kete/proxy.go` (plan 001 phase 4).
- **Depends-on:** `[]`
- **Done when:** `curl localhost:8080/health` → 200.

### Phase 2 — Graceful shutdown

- **Outcome:** SIGINT closes within 500 ms; no orphan listeners.
- **Slice:** `http.Server.Shutdown(ctx)`; signal handler.
- **Context:** `internal/proxy/server.go`.
- **Done when:** integration test sends SIGINT mid-request, asserts < 500 ms.

### Phase 3 — Body limit + timeout

- **Outcome:** Bodies > 10 MB → 413; 5-min timeout enforced.
- **Slice:** `http.MaxBytesReader`; `context.WithTimeout`.
- **Context:** `internal/proxy/server.go`; brief 002 Constraints.
- **Depends-on:** `[phase-1]`
- **Done when:** 11 MB → 413; slow upstream → timeout.

### Phase 4 — Header whitelist + redaction

- **Outcome:** Forward only `[x-api-key, authorization, anthropic-version, content-type, anthropic-beta]`; logs redact secrets.
- **Slice:** `internal/proxy/headers.go` with `Sanitise(http.Header) http.Header` + `RedactForLog`.
- **Context:** brief 002 Constraints.
- **Depends-on:** `[]`
- **Done when:** unit test asserts whitelist + redaction.

### Phase 5 — Raw passthrough forwarder (anthropic-direct)

- **Outcome:** `POST /v1/messages` body read once, forwarded byte-identical, response streamed back.
- **Slice:** `internal/proxy/forward.go`; no `httputil.ReverseProxy` (it buffers).
- **Context:** ADR 0006; `internal/proxy/headers.go`; `internal/proxy/server.go`.
- **Depends-on:** `[phase-1, phase-4]`
- **Done when:** httptest upstream sees byte-identical request bytes; response streams back unchanged.

### Phase 6 — SSE streaming

- **Outcome:** Each chunk flushed as it arrives; inter-chunk timing mirrors upstream.
- **Slice:** `http.Flusher` after each read; tee'd `io.Reader`.
- **Context:** `internal/proxy/forward.go`.
- **Depends-on:** `[phase-5]`
- **Done when:** integration test asserts arrival times.

### Phase 7 — `Adapter` interface

- **Outcome:** `Adapter.Forward(ctx, headers, rawBody, w) error`; anthropic-direct is first impl wrapping phase 5.
- **Slice:** `internal/adapter/adapter.go` + `internal/adapter/anthropic.go`.
- **Context:** ADR 0007; `internal/proxy/forward.go`.
- **Depends-on:** `[phase-5, phase-6]`
- **Done when:** Forward path goes through the adapter; tests still pass.

### Phase 8 — Upstream selector

- **Outcome:** `SelectUpstream(headers, body) (name, error)` per ADR 0015: header > model-id pattern > env. `x-kete-upstream` consumed and stripped.
- **Slice:** `internal/proxy/route.go`.
- **Context:** ADR 0015; `internal/proxy/headers.go`.
- **Depends-on:** `[phase-4]`
- **Done when:** unit test covers all three precedence rules.

### Phase 9 — Body-injection helper (messages array)

- **Outcome:** `InjectAtMessages(rawBody, segment) ([]byte, error)` splices before closing `]` of `messages`; result still parses; prefix bytes unchanged.
- **Slice:** `internal/proxy/inject.go`; raw-byte scanner.
- **Context:** ADR 0006, ADR 0013.
- **Depends-on:** `[]`
- **Done when:** test asserts byte-for-byte prefix equality.

### Phase 10 — Body-injection helper (cache breakpoint)

- **Outcome:** `InjectBeforeCacheBreakpoint(rawBody, segment) ([]byte, error)` finds first `cache_control` and inserts ahead of it.
- **Slice:** `internal/proxy/inject_cache.go`.
- **Context:** ADR 0006; `internal/proxy/inject.go`.
- **Depends-on:** `[phase-9]`
- **Done when:** test asserts prefix-hash stable across two injections.

### Phase 11 — Capture middleware

- **Outcome:** After response completes, async write of a raw `tasks` row (no extraction yet).
- **Slice:** `internal/proxy/capture.go`; goroutine; uses `store.CreateTask`.
- **Context:** `internal/store/tasks.go` (plan 004 phase 8); `internal/proxy/forward.go`.
- **Depends-on:** `[phase-7]`
- **Done when:** 1 request → 1 row in DB after response; proxy timing unchanged.

### Phase 12 — Project-path resolution at capture

- **Outcome:** Resolves `project_path` from `KETE_PROJECT` or `cwd`; written into the row.
- **Slice:** `internal/proxy/project.go`.
- **Context:** `internal/proxy/capture.go`.
- **Depends-on:** `[phase-11]`
- **Done when:** test asserts the row's `project_path` matches expectation.

### Phase 13 — Injection middleware (no-op ranking)

- **Outcome:** On request, fetches `ListTasks(project_path)` top 3 and splices via phase 10. Skipped when empty. Ranking is "newest" — real ranking lands in plan 010.
- **Slice:** `internal/proxy/inject_mw.go`.
- **Context:** `internal/store/tasks.go` (plan 004 phase 11); `internal/proxy/inject_cache.go`; `internal/proxy/project.go`.
- **Depends-on:** `[phase-10, phase-12]`
- **Done when:** seeded tasks → second request body contains injected segment; prefix bytes still cache-stable.

### Phase 14 — Drift hook

- **Outcome:** Every Nth prompt (`KETE_DRIFT_CHECK_INTERVAL`, default 5) fires a `drift.Check` event; consumer is no-op.
- **Slice:** `internal/proxy/drift_hook.go`; channel + per-session counter.
- **Context:** brief 007 Constraints.
- **Depends-on:** `[phase-7]`
- **Done when:** test asserts hook fires on prompt 5, 10, 15.

### Phase 15 — Compaction hook

- **Outcome:** Usage tap fires `compact.PreCompute` at warning, `compact.Apply` at clear; no-op consumers.
- **Slice:** `internal/proxy/compact_hook.go`.
- **Context:** brief 008 Constraints.
- **Depends-on:** `[phase-7]`
- **Done when:** stubbed usage crosses thresholds; events fire correctly.

### Phase 16 — `kete_expand` loop guard

- **Outcome:** Hard cap of 5 cycles per request.
- **Slice:** `internal/proxy/expand_loop.go`.
- **Context:** brief 002 Constraints.
- **Depends-on:** `[phase-7]`
- **Done when:** 6th cycle short-circuits.

### Phase 17 — Doc: `docs/reference/proxy.md`

- **Outcome:** Endpoint surface, env vars, body-limit, header whitelist, redaction.
- **Slice:** new file.
- **Context:** brief 002 Doc impact.
- **Depends-on:** `[phase-3, phase-4]`
- **Done when:** file exists; linked from README.

### Phase 18 — Doc: `docs/explanation/why-proxy-not-just-mcp.md`

- **Outcome:** Cooperative-vs-not argument; the table from the brief.
- **Slice:** new file.
- **Context:** brief 002 (Why a proxy and not just MCP).
- **Depends-on:** `[]`
- **Done when:** file exists; cited from `docs/reference/mcp.md` (plan 003 phase 8).

## Out of scope

- Extraction (plan 011), full drift (plan 007), full compaction (plan 008), full ranking (plan 010), MCP (plan 003), bedrock (plan 012), cc-proxy (plan 013), extended-cache (plan 009).
- OpenAI `/v1/responses`. Non-localhost binding.

## Assumptions

- chi sufficient (ADR 0005). Crush sends standard headers. The splice points stay stable through 2026-Q2; if not, ADR 0006 forces revisit.
