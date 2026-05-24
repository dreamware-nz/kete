# kete

> A local memory and reasoning layer for AI coding sessions, sized for
> the dreamware-nz workflow on Crush. Captures reasoning between turns,
> persists it locally, and injects it into new sessions so they don't
> start from zero.
>
> Name from te reo Māori — `kete` is a woven basket; `ngā kete o te
> wānanga` is the canonical metaphor for collected, portable knowledge.

## Quickstart

```sh
make build                          # builds bin/kete
./bin/kete doctor                   # check ~/.kete + upstream
./bin/kete proxy                    # start the local HTTP proxy (WIP)
./bin/kete status                   # show captured tasks for cwd
./bin/kete tasks "auth flow"        # search captured reasoning
```

Point Crush at the proxy via `ANTHROPIC_BASE_URL=http://127.0.0.1:8765`
(default port; change in `kete proxy --help`). Everything else stays
the same.

## What it is

- **HTTP proxy** between Crush and one of three upstreams
  (`anthropic` | `cc-proxy` | `bedrock`; ADR 0015) that captures
  reasoning per turn and injects relevant prior reasoning per prompt.
- **stdio MCP server** with two tools (`kete_preview`, `kete_expand`)
  for clients that prefer the cooperative path.
- **SQLite memory store** at `~/.kete/memory.db` (overridable via
  `KETE_HOME` or `KETE_DB_PATH`). We own the schema (ADR 0003).

## Status

Pre-`0.1.0`. The store and CLI shell ship; the proxy and MCP server
are wiring up. See `process/plans/000-kete-overview.md` for the full
plan and current phase.

## Layout

- `cmd/kete/` — the binary entry point.
- `cmd/ketedoc/` — `make docs` regenerates `docs/reference/cli.md`.
- `internal/cli/` — cobra command tree.
- `internal/store/` — SQLite memory store + migrations.
- `process/` — briefs, ADRs, plans (read these before changing the
  shape of anything).
- `docs/` — Diátaxis-shaped: `tutorials/`, `how-to/`, `reference/`,
  `explanation/`.

## License

Apache-2.0 (matches `dreamware-nz/cc-proxy`).
