# kete

> A local memory and reasoning layer for AI coding sessions, sized for
> the dreamware-nz workflow on Crush. Captures reasoning between turns,
> persists it locally, and injects it into new sessions so they don't
> start from zero.
>
> Name from te reo Māori — `kete` is a woven basket; `ngā kete o te
> wānanga` is the canonical metaphor for collected, portable knowledge.

## Install

One-liner (macOS, Linux; downloads the latest release into `~/.local/bin`):

```sh
curl -fsSL https://raw.githubusercontent.com/dreamware-nz/kete/main/install.sh | sh
```

Or build from source:

```sh
git clone git@github.com:dreamware-nz/kete.git
cd kete
make install                        # builds and installs into ~/.local/bin/kete
```

`make build` puts the binary at `bin/kete` if you'd rather run it
out of the tree.

## Quickstart

```sh
kete doctor                         # check ~/.kete + upstream
kete proxy                          # start the local HTTP proxy
kete status                         # captured tasks for cwd
kete tasks "auth flow"              # search captured reasoning
```

Point Crush at the proxy:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:8080
```

That's it. Everything else stays the same.

## What it is

- **HTTP proxy** on `127.0.0.1:8080` that forwards `POST /v1/messages`
  byte-exact to one of three upstreams: `anthropic` | `cc-proxy` |
  `bedrock`. Per-request selection via header > model-id > env
  (ADR 0015). Captures every turn, splices prior memory before
  forwarding, scores drift between turns, queues corrections,
  rewrites the body when the context window fills.
- **stdio MCP server** (`kete mcp`) with `kete_preview` and
  `kete_expand` tools. Belt-and-braces for models that prefer the
  cooperative path. Cross-process resolution: a memory injected by
  the proxy is expandable by an MCP server in another process via
  the shared 8-char short id.
- **SQLite memory store** at `~/.kete/memory.db`. Pure-Go, no cgo
  (ADR 0002). Tables: `tasks`, `steps`, `drift_log`, `sync_tracker`,
  `injection_log`. Override with `KETE_HOME` or `KETE_DB_PATH`.

## Documentation

- **Tutorial** — `docs/tutorials/first-run.md`
- **How-to** — `docs/how-to/{use-bedrock,use-cc-proxy,inspect-memory,enable-extended-cache}.md`
- **Reference** — `docs/reference/{cli,env,proxy,mcp,schema}.md`
- **Explanation** — `docs/explanation/{why-proxy-not-just-mcp,raw-body-preservation,three-upstreams}.md`

## Status

`0.1.0` — first release. Brief 000 success criteria all met,
live-verified end-to-end against AWS Bedrock + Anthropic Claude
Haiku 4.5 (memory injection round-trip, expand-loop tool dispatch,
streaming SSE). See `CHANGELOG.md` for the honest gaps list.

## Layout

- `cmd/kete/` — binary entry point.
- `cmd/ketedoc/` — `make docs` regenerates `docs/reference/cli.md`.
- `internal/cli/` — cobra command tree.
- `internal/proxy/` — the HTTP proxy + handlers.
- `internal/adapter/{anthropic,bedrock,ccproxy}/` — per-upstream wire.
- `internal/inject/` — byte-offset body edits + the shared shortID.
- `internal/extract/` — Haiku-backed extractor.
- `internal/drift/` — score, level, persist.
- `internal/compact/` — usage tap, Summary, Apply.
- `internal/keepalive/` — extended-cache manager.
- `internal/mcp/` — stdio JSON-RPC server.
- `internal/store/` — SQLite memory store + migrations.
- `process/` — briefs, ADRs, plans (read before changing shape).
- `docs/` — Diátaxis-shaped.

## License

Apache-2.0 (matches `dreamware-nz/cc-proxy`).
