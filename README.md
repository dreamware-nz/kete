# kete

[![ci](https://github.com/dreamware-nz/kete/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/dreamware-nz/kete/actions/workflows/ci.yml)
[![release](https://github.com/dreamware-nz/kete/actions/workflows/release.yml/badge.svg)](https://github.com/dreamware-nz/kete/actions/workflows/release.yml)
[![latest release](https://img.shields.io/github/v/release/dreamware-nz/kete?sort=semver&display_name=tag)](https://github.com/dreamware-nz/kete/releases/latest)
[![go reference](https://pkg.go.dev/badge/github.com/dreamware-nz/kete.svg)](https://pkg.go.dev/github.com/dreamware-nz/kete)
[![go report](https://goreportcard.com/badge/github.com/dreamware-nz/kete)](https://goreportcard.com/report/github.com/dreamware-nz/kete)
[![license](https://img.shields.io/github/license/dreamware-nz/kete)](LICENSE)

> Local memory for AI coding sessions. Stops your assistant from
> forgetting what it figured out yesterday.

`kete` is a local HTTP proxy and MCP server that sits between your
AI coding tool (Claude Code, Crush, Cursor, anything that talks the
Anthropic Messages API) and the upstream model. It does four things
the model and the IDE alone can't:

1. **Capture.** When a turn ends, it extracts the goal, key
   decisions, and files touched, and persists them to a local
   SQLite store.
2. **Inject.** When you start a new prompt, relevant prior reasoning
   for this project is spliced into the request *before* the model
   sees it. Your assistant doesn't have to re-investigate.
3. **Detect drift.** It scores each turn's actions against the
   stated goal. When the score drops, it injects a correction into
   the next request.
4. **Auto-compact.** When the context window fills, it replaces the
   conversation with a structured summary that preserves the goal,
   decisions, and current state.

Name from te reo Māori — `kete` is a woven basket; `ngā kete o te
wānanga` is the canonical metaphor for collected, portable
knowledge.

> Built and tuned for [Crush](https://charmbracelet.com/crush) on the
> dreamware-nz workflow, but the wire surfaces are protocol-compliant
> so it works with any Anthropic-shaped client.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/dreamware-nz/kete/main/install.sh | sh
```

Drops `kete` in `~/.local/bin`. Override with `PREFIX`:

```sh
curl -fsSL https://raw.githubusercontent.com/dreamware-nz/kete/main/install.sh \
  -o /tmp/install.sh
PREFIX=/usr/local sh /tmp/install.sh
```

The installer detects OS/arch, downloads the matching binary from
the latest GitHub Release, verifies SHA256, and warns if the
install dir isn't on your `PATH`.

Or build from source:

```sh
git clone https://github.com/dreamware-nz/kete.git
cd kete
make install                    # ~/.local/bin (override with PREFIX=)
```

`make build` puts the binary at `bin/kete` if you'd rather run it
out of the tree.

Supported: `darwin/{amd64,arm64}` and `linux/{amd64,arm64}`. Pure
Go, no cgo.

## Quickstart

```sh
kete doctor                         # check ~/.kete + upstream
kete proxy                          # start the local HTTP proxy
```

Point your client at the proxy:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:8080
```

Then run your usual session. After a few turns:

```sh
kete status                         # captured tasks for cwd
kete tasks "auth flow"              # search captured reasoning
```

That's it. Your next session against the same project starts with
the prior session's reasoning already in context.

## Upstreams

Three upstreams are supported per request, selected by header,
model-id, or env var:

- **anthropic** — `https://api.anthropic.com` with your own
  `ANTHROPIC_API_KEY`. Default.
- **bedrock** — AWS Bedrock with SigV4 signing. Set
  `KETE_UPSTREAM=bedrock` and `AWS_REGION`. See
  [`docs/how-to/use-bedrock.md`](docs/how-to/use-bedrock.md).
- **cc-proxy** — The
  [`dreamware-nz/cc-proxy`](https://github.com/dreamware-nz/cc-proxy)
  macOS menubar app, which maps your Claude Code subscription to
  an Anthropic-compatible HTTP server. See
  [`docs/how-to/use-cc-proxy.md`](docs/how-to/use-cc-proxy.md).

Per-request override via `x-kete-upstream` header.

## What it isn't

- **Not a hosted service.** Everything runs locally. No telemetry.
- **Not a vendor lock-in.** The proxy is byte-exact on the request
  path, so swapping kete in or out is invisible to the model.
- **Not a team-sync system.** v1 is single-user. A team backend is
  a future brief.
- **Not a fork or port of any other project.** The reasoning-capture
  problem is well-trodden territory; the design draws on prior art
  but ships its own protocol surfaces, schemas, and tooling.

## Documentation

The docs follow [Diátaxis](https://diataxis.fr):

- **Tutorial** — [first-run](docs/tutorials/first-run.md)
- **How-to** — [bedrock](docs/how-to/use-bedrock.md) ·
  [cc-proxy](docs/how-to/use-cc-proxy.md) ·
  [inspect memory](docs/how-to/inspect-memory.md) ·
  [extended cache](docs/how-to/enable-extended-cache.md)
- **Reference** — [CLI](docs/reference/cli.md) ·
  [proxy](docs/reference/proxy.md) ·
  [MCP](docs/reference/mcp.md) ·
  [schema](docs/reference/schema.md) ·
  [env](docs/reference/env.md)
- **Explanation** — [why a proxy, not just MCP](docs/explanation/why-proxy-not-just-mcp.md) ·
  [raw-body preservation](docs/explanation/raw-body-preservation.md) ·
  [three upstreams](docs/explanation/three-upstreams.md)

## Status

`v0.1.0`. The four core capabilities are wired and live-verified
end-to-end against AWS Bedrock + Anthropic Claude Haiku 4.5
(memory injection round-trip, expand-loop tool dispatch, streaming
SSE). 80+ tests cover the proxy, adapters, MCP server, drift, and
compaction. See [`CHANGELOG.md`](CHANGELOG.md) for the honest gaps
list.

## How it's built

- Single static binary, pure Go, no cgo. SQLite via
  [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite).
- HTTP server is [`net/http`](https://pkg.go.dev/net/http) +
  [`go-chi/chi`](https://github.com/go-chi/chi).
- AWS Bedrock signing via `aws-sdk-go-v2`.
- MCP stdio is hand-rolled JSON-RPC 2.0 (~200 LOC).

The [`process/`](process) directory contains briefs, ADRs, and
plans — read them before changing the shape of anything. The
project follows the chain `idea → brief → ADR(s) → plan (phased)
→ execute`, with the templates and skills documented in
[`github.com/dreamware-nz/process`](https://github.com/dreamware-nz/process).

## License

[Apache-2.0](LICENSE).

## Acknowledgements

- The reasoning-capture pattern is convergent prior art; we're not
  the first to build something in this shape, and we owe a debt to
  the projects that walked it earlier.
- Built for [Crush](https://charmbracelet.com/crush) (Charm) and
  composes with
  [`dreamware-nz/cc-proxy`](https://github.com/dreamware-nz/cc-proxy).
