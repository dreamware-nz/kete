# Changelog

All notable changes to kete. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## 0.1.0 — 2026-05-24

First release. Brief 000 master plan complete.

### Added

- **CLI** (`kete`): `proxy`, `mcp`, `status`, `tasks <q>`, `drift-test`,
  `doctor`, `purge`, `--version`, `--help`. (Plan 001.)
- **Memory store** (`~/.kete/memory.db`): SQLite via pure-Go
  `modernc.org/sqlite`. Tables: `tasks`, `steps`, `drift_log`,
  `sync_tracker`, `injection_log`. Numbered migrations applied at
  startup. (Plan 004 / ADR 0003.)
- **HTTP proxy** on `127.0.0.1:8080`. Routes `POST /v1/messages` byte-
  exact (ADR 0006) to the configured upstream; captures every request;
  splices prior memory before forwarding; redacts secrets in logs.
  Three upstreams supported per request: anthropic-direct, cc-proxy,
  Bedrock. (Plan 002 / ADRs 0005, 0006, 0007, 0015.)
- **MCP stdio server** with `kete_preview` and `kete_expand` tools,
  hand-rolled JSON-RPC 2.0 (ADR 0012). Tool descriptions kete-authored
  (ADR 0008).
- **Drift detection**: 0–10 score → 4-level correction (none / nudge /
  correct / intervene / halt) with per-session escalation counter and
  forced-recovery threshold. Steps vs `drift_log` persistence by score.
  (Plan 007 / ADR 0011.)
- **Auto-compaction**: warn / clear thresholds, background pre-compute
  via Haiku, structured Summary type. Apply path is a TODO until first
  user. (Plan 008.)
- **Extended cache (`--extended-cache`)**: opt-in 60-second keep-alive
  per ADR 0013. Stash-on-forward; ticker fires once a minute; per-
  session cap of 2 keep-alives per idle period; idle threshold 4 min;
  total max idle 10 min. (Plan 009 / ADR 0013.)
- **Bedrock adapter**: AWS SigV4 signing via `aws-sdk-go-v2`,
  Anthropic-shaped body translated for Bedrock per ADR 0014, response
  event-stream demuxed into SSE for clients. (Plan 012 / ADR 0014.)
- **cc-proxy adapter**: wire-identical to anthropic, base URL +
  shared-secret variant. (Plan 013.)
- **Haiku-backed extraction client** with embedded prompts for task,
  decisions, drift score, drift correction, and compact summary. 5xx
  retry with exponential backoff; 4xx fail-fast. Per-call-site
  `max_tokens`. (Plan 011 / ADR 0009.)
- **Docs** (Diátaxis): tutorial `first-run.md`; reference `cli.md`,
  `proxy.md`, `mcp.md`, `schema.md`, `env.md`; explanation
  `why-proxy-not-just-mcp.md`.

### Notes

- Pure-Go SQLite (`modernc.org/sqlite`); no cgo.
- One static binary per OS/arch; `make build` for local dev,
  `make docs` to regenerate `docs/reference/cli.md`.
- Apache-2.0.

[0.1.0]: https://github.com/dreamware-nz/kete/releases/tag/v0.1.0
