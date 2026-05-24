---
id: 013-cc-proxy-upstream
date: 2026-05-24
status: done
brief: 013-cc-proxy-upstream
design: null
adrs: [0015-three-upstreams-selection]
---

# 013 — cc-proxy upstream

## Goal

Treat cc-proxy as a base-URL + auth-header configuration of the existing Anthropic adapter; no new adapter, no duplicated functionality.

## Phases

### Phase 1 — Env config

- **Outcome:** `KETE_CC_PROXY_URL` (default `http://127.0.0.1:8787`) and `KETE_CC_PROXY_KEY` read at startup; missing key with `KETE_UPSTREAM=cc-proxy` errors with a clear message.
- **Slice:** `internal/adapter/anthropic/ccproxy.go`.
- **Context:** brief 013 Constraints; ADR 0015.
- **Depends-on:** `[]`
- **Done when:** missing-key test fails fast with named error.

### Phase 2 — Adapter parameterisation

- **Outcome:** Plan 002 phase 7 anthropic adapter takes `(baseURL, authHeader)`; same code serves direct API and cc-proxy.
- **Slice:** refactor `internal/adapter/anthropic.go`.
- **Context:** `internal/adapter/anthropic.go`; brief 013 Constraints.
- **Depends-on:** `[]`
- **Done when:** existing direct-API tests still pass; cc-proxy variant boots.

### Phase 3 — Wire selector

- **Outcome:** `SelectUpstream` routes `KETE_UPSTREAM=cc-proxy` and `x-kete-upstream: cc-proxy`; auto-detect never picks cc-proxy.
- **Slice:** edit `internal/proxy/route.go`.
- **Context:** `internal/proxy/route.go` (plan 002 phase 8); ADR 0015.
- **Depends-on:** `[phase-1, phase-2]`
- **Done when:** unit test covers header + env paths; auto-detect refuses cc-proxy.

### Phase 4 — Auth header swap

- **Outcome:** Outbound `x-api-key` is `$KETE_CC_PROXY_KEY`, never the inbound user key.
- **Slice:** edit `internal/adapter/anthropic/ccproxy.go`.
- **Context:** `internal/adapter/anthropic.go`; `internal/proxy/headers.go`.
- **Depends-on:** `[phase-2]`
- **Done when:** test asserts the outbound key is the cc-proxy key.

### Phase 5 — Cache prefix preservation test

- **Outcome:** Same input → same prefix bytes whether direct or cc-proxy.
- **Slice:** integration test asserting prefix-bytes hash equality.
- **Context:** ADR 0006; `internal/adapter/anthropic.go`.
- **Depends-on:** `[phase-3, phase-4]`
- **Done when:** hash equal.

### Phase 6 — `kete doctor` cc-proxy check

- **Outcome:** When configured, doctor pings `$KETE_CC_PROXY_URL/health`; PASS/FAIL with underlying error.
- **Slice:** edit `cmd/kete/doctor.go`.
- **Context:** `cmd/kete/doctor.go` (plan 001 phases 9–10); `internal/adapter/anthropic/ccproxy.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** stopped cc-proxy → FAIL with refused; running → PASS.

### Phase 7 — No-retry assertion

- **Outcome:** Adapter targeting cc-proxy passes 5xx straight through; no retry/fallback.
- **Slice:** test in `internal/adapter/anthropic/ccproxy_test.go`.
- **Context:** brief 013 Constraints (do not duplicate cc-proxy features).
- **Depends-on:** `[phase-2]`
- **Done when:** stub returns 503 once → client sees 503.

### Phase 8 — Capture preserves user-requested model

- **Outcome:** `task.system_name` is the model the user requested (read from request body), regardless of cc-proxy fallback.
- **Slice:** edit `internal/proxy/capture.go`.
- **Context:** `internal/proxy/capture.go` (plan 002 phase 11).
- **Depends-on:** `[phase-2]`
- **Done when:** test: cc-proxy synthesised OpenAI fallback response → captured model is the Anthropic id.

### Phase 9 — Doc: `docs/how-to/use-cc-proxy.md`

- **Outcome:** Install cc-proxy, copy key, env, run.
- **Slice:** new file.
- **Context:** brief 013 Doc impact.
- **Depends-on:** `[]`
- **Done when:** file exists.

### Phase 10 — Doc: `docs/explanation/grov-and-cc-proxy.md`

- **Outcome:** Responsibility-boundary table + examples.
- **Slice:** new file.
- **Context:** brief 013 (Boundary section).
- **Depends-on:** `[]`
- **Done when:** file exists; README quickstart updated.

## Out of scope

- Bundling or auto-starting cc-proxy. OAuth handling. Reading cc-proxy's `usage.json`. Multiple cc-proxy instances.

## Assumptions

- cc-proxy `/v1/messages` is wire-identical to Anthropic's. Plan 012's selector extension lands first or rebases.
