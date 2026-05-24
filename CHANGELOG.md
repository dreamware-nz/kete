# Changelog

All notable changes to kete. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## Unreleased

Master plan 000 substantially landed but **not yet shipped**. Brief 000
success criterion #1 (captured task includes goal/decisions/files)
holds; criterion #2 (next session receives prior task) holds;
criterion #3 (drift-test produces coherent output) is wired and
verified end-to-end against a stubbed Haiku, but never exercised
against a real one.

### Added

- **CLI** (`kete`): `proxy`, `mcp`, `status`, `tasks <q>`, `drift-test`,
  `doctor`, `purge`, `--version`, `--help`. (Plan 001.)
- **Memory store** (`~/.kete/memory.db`): SQLite via pure-Go
  `modernc.org/sqlite`. Tables: `tasks`, `steps`, `drift_log`,
  `sync_tracker`, `injection_log`. Numbered migrations applied at
  startup. Single connection (writers serialised) and sub-second
  timestamps for stable ordering. (Plan 004 / ADR 0003.)
- **HTTP proxy** on `127.0.0.1:8080`. Routes `POST /v1/messages` byte-
  exact (ADR 0006) to the configured upstream. Per request: read body,
  select upstream, splice prior memory + queued correction (if any),
  forward, capture (pre-injection body), enrich captured row via
  Haiku ExtractTask in the background, score every Nth request for
  drift and queue correction for the next request. (Plan 002 / ADRs
  0005, 0006, 0007, 0011, 0015.)
- **MCP stdio server** with `kete_preview` and `kete_expand` tools,
  hand-rolled JSON-RPC 2.0 (ADR 0012). Tool descriptions
  kete-authored (ADR 0008).
- **Drift detection**: 0–10 score → 4-level correction (none / nudge /
  correct / intervene / halt) with per-session escalation counter and
  forced-recovery threshold. Steps vs `drift_log` persistence by
  score. End-to-end test verifies the Nth-request hook fires, queues
  a correction, and the (N+1)th request body the upstream sees
  contains the correction segment. (Plan 007 / ADR 0011.)
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
- **Compaction layer**: `compact.Compute` (Haiku via
  `compact_summary.txt`), `Cache` with background pre-compute, and
  `Apply` that rewrites the request body to `[{role:user,
  content:<summary>}, {role:user, content:<next prompt>}]`. Tested
  against stubbed Haiku.
- **Docs** (Diátaxis): tutorial `first-run.md`; reference `cli.md`,
  `proxy.md`, `mcp.md`, `schema.md`, `env.md`; explanation
  `why-proxy-not-just-mcp.md`.

### Honest gaps

- **Compaction trigger never fires.** The `compact.Apply` path is
  built and tested in isolation, but the proxy doesn't yet have a
  streaming-response token-usage tap (parses Anthropic's
  `event: message_delta` frames and pulls `delta.usage`). Without it
  the threshold cross never trips. **Plan 008 status remains
  `active`.**
- **Drift fixture set deferred.** Plan 007 ships the scoring and
  correction loop; it does not ship a 20-fixture hand-labelled set
  to calibrate threshold accuracy against. That work needs a real
  Haiku endpoint and judgement calls.
- **Bedrock smoke-untested.** SigV4 + event-stream demux compile and
  pass shape tests but have not been exercised against
  `bedrock-runtime`. AWS credentials live in the user's environment;
  the byte-translation logic is what we own and that is tested.
- **cc-proxy smoke-untested.** Shape tests only; no live cc-proxy
  round-trip.
- **MCP cache not shared with proxy injection.** Plan 010 phase 6 —
  the proxy's inline `buildMemoryPayload` still runs separately from
  MCP's preview cache. Two paths today, one path tomorrow.
- **Capture pipeline collapsed to a single source ("proxy").** Brief
  006's five-source vision (proxy + Cursor/Zed/Codex/Antigravity
  scanners) deferred until a non-Crush user appears. Documented in
  `internal/capture/capture.go` and `process/drift.md`.

### Tooling

- Pure-Go SQLite (`modernc.org/sqlite`); no cgo.
- One static binary per OS/arch; `make build` for local dev,
  `make docs` to regenerate `docs/reference/cli.md`,
  `make release` builds darwin/{amd64,arm64} and
  linux/{amd64,arm64}.
- Apache-2.0.

[Unreleased]: https://github.com/dreamware-nz/kete
