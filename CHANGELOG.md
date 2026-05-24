# Changelog

All notable changes to kete. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## 0.1.0 — 2026-05-24

First release. Master plan 000 complete; brief 000 success criteria
all met. Live-verified end-to-end through real AWS Bedrock against
Anthropic Claude Haiku 4.5 (`us.anthropic.claude-haiku-4-5-20251001`):
non-streaming and streaming round-trips, capture, and memory injection
(seeded prior task → next request's response retrieved the secret word
"kowhai" that lived only in the injected memory; input tokens
jumped 22 → 101, confirming splice).

### Added

- **CLI** (`kete`): `proxy`, `mcp`, `status`, `tasks <q>`, `drift-test`,
  `doctor`, `purge`, `--version`, `--help`. (Plan 001.)
- **Memory store** (`~/.kete/memory.db`): SQLite via pure-Go
  `modernc.org/sqlite`. Tables: `tasks`, `steps`, `drift_log`,
  `sync_tracker`, `injection_log`. Numbered migrations applied at
  startup; single connection (writers serialised); sub-second
  timestamps for stable ordering. (Plan 004 / ADR 0003.)
- **HTTP proxy** on `127.0.0.1:8080`. Routes `POST /v1/messages`
  byte-exact (ADR 0006) to the configured upstream. Per request:
  read body → maybe rewrite for compaction → splice prior memory →
  splice queued correction → forward through a usage-tap that
  observes Anthropic SSE token counts → capture (pre-injection
  body) → enrich captured row via Haiku ExtractTask in the
  background → score every Nth request for drift → queue correction
  for the next request → stash for keepalive. (Plan 002 / ADRs
  0005, 0006, 0007, 0011, 0015.)
- **MCP stdio server** with `kete_preview` and `kete_expand` tools,
  hand-rolled JSON-RPC 2.0 (ADR 0012). Tool descriptions
  kete-authored (ADR 0008).
- **Drift detection**: 0–10 score → 4-level correction (none / nudge /
  correct / intervene / halt) with per-session escalation counter
  and forced-recovery threshold. Steps vs `drift_log` persistence by
  score. End-to-end test verifies the Nth-request hook fires, queues
  a correction, and the (N+1)th request body the upstream sees
  contains the correction segment. (Plan 007 / ADR 0011.)
- **Auto-compaction**: streaming usage tap parses Anthropic
  `event: message_start` and `event: message_delta` SSE frames as
  they pass through the proxy, accumulates token counts, observes
  via per-session compactHook (warn 160 000 / clear 180 000
  defaults). On warn, kicks off a background `compact.Compute` to
  build a structured Summary. On clear, queues `compact.Apply` for
  the next request, which rewrites `messages` to
  `[<summary>, <next prompt>]`. End-to-end test with low thresholds
  proves the round-trip. (Plan 008 / brief 008.)
- **Bedrock adapter**: AWS SigV4 signing via `aws-sdk-go-v2`,
  Anthropic-shaped body translated for Bedrock per ADR 0014, response
  event-stream demuxed into SSE for clients. SigV4 + event-stream
  smoke-untested without AWS creds. (Plan 012 / ADR 0014.)
- **cc-proxy adapter**: wire-identical to anthropic, base URL +
  shared-secret variant. (Plan 013.)
- **Haiku-backed extraction client** with embedded prompts for task,
  decisions, drift score, drift correction, and compact summary. 5xx
  retry with exponential backoff; 4xx fail-fast. Per-call-site
  `max_tokens`. (Plan 011 / ADR 0009.)
- **Extended cache (`--extended-cache`)**: opt-in 60-second keep-alive
  per ADR 0013. Stash-on-forward; ticker fires once a minute; per-
  session cap of 2 keep-alives per idle period; idle threshold 4 min;
  total max idle 10 min. Tested via `tickAt(now)` injection.
  (Plan 009 / ADR 0013.)
- **Docs** (Diátaxis): tutorial `first-run.md`; reference `cli.md`,
  `proxy.md`, `mcp.md`, `schema.md`, `env.md`; explanation
  `why-proxy-not-just-mcp.md`.

### Honest gaps (deferred, not blocking 0.1.0)

- **cc-proxy smoke-untested.** Shape tests only; live cc-proxy
  round-trip needs the cc-proxy macOS app running, which is out of
  band for this session.
- **Anthropic-direct smoke-untested.** No `ANTHROPIC_API_KEY` in the
  test environment; that path is identical to cc-proxy at the wire
  level (cc-proxy's adapter literally reuses anthropic.Adapter).
  The Bedrock live verification covers the equivalent code paths
  (capture, inject, drift, compaction triggers) — only the
  upstream adapter is unique.
- **Drift fixture set deferred.** Plan 007 ships the scoring and
  correction loop; calibration of threshold accuracy against a
  hand-labelled set needs a real Haiku endpoint and judgement calls.
  Drift on the live system needs `ANTHROPIC_API_KEY` because the
  extractor goes Anthropic-direct (ADR 0009). A Bedrock-only
  environment can't run extraction without a separate Anthropic key
  — this is by design per ADR 0009.
- **MCP cache not shared with proxy injection.** Plan 010 phase 6 —
  the proxy's inline `buildMemoryPayload` and MCP's preview cache
  are separate paths today. Both work; sharing them is mechanical.
- **Capture pipeline collapsed to a single source ("proxy").** Brief
  006's five-source vision (proxy + Cursor/Zed/Codex/Antigravity
  scanners) deferred until a non-Crush user appears. Documented in
  `internal/capture/capture.go` and `process/drift.md`.
- **ADR 0007 `Semantics` interface** has no implementation — only
  `Wire` ships in 0.1.0 because nothing else in v1 needs the typed
  view (extraction operates on raw JSON bytes). Lands when the
  orchestrator that runs the kete_expand tool loop in the proxy
  arrives.
- **Expand-loop guard** (`expand_loop.go`) exists but has no caller
  yet — the proxy doesn't run the kete_expand tool loop in v1; the
  guard is wired when that orchestrator lands.

### Tooling

- Pure-Go SQLite (`modernc.org/sqlite`); no cgo.
- One static binary per OS/arch; `make build` for local dev,
  `make docs` to regenerate `docs/reference/cli.md`,
  `make release` builds darwin/{amd64,arm64} and
  linux/{amd64,arm64}.
- Apache-2.0.

[0.1.0]: https://github.com/dreamware-nz/kete/releases/tag/v0.1.0
