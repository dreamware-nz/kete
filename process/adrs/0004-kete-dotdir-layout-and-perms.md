---
number: 0004
title: kete dotdir layout — ~/.kete/, KETE_* env vars, x-kete-* headers
date: 2026-05-24
status: accepted
brief: 004-memory-store
supersedes: null
superseded-by: null
---

# 0004 — kete dotdir layout — ~/.kete/, KETE_* env vars, x-kete-* headers

## Context

kete needs a dotdir, env-var prefix, and HTTP header namespace. With no compat constraint inherited from grov (ADR 0000), these are ours to pick. The constraints are: greppable, short, no collision with other tools dreamware-nz runs, and consistent across the corpus.

## Decision

**Filesystem layout.**

- `~/.kete/` — directory mode `0700`.
- `~/.kete/memory.db` — SQLite store, file mode `0600`. Override via `KETE_DB_PATH` (used by tests).
- `~/.kete/kete-proxy.log` — proxy log (when `--debug`). Mode umask-derived; we don't loosen if pre-existing.
- `~/.kete/kete-mcp.log` — MCP server log. Same.
- `~/.kete/credentials.json` — reserved name for the future cloud-sync brief; not created in v1.

On first creation, the directory is `mkdir -p` mode `0700`; files use `os.OpenFile` with mode `0600`. A `chmod` after creation handles the case where the file pre-existed.

We do NOT introduce an XDG-style `~/.local/share/kete/` path. `~/.kete/` is short, greppable, consistent with cc-proxy's `~/Library/Application Support/cc-proxy/` choice for *its* world (Mac app), and fine for our world (Go CLI on macOS/Linux).

**Environment variables.** All kete env vars use the `KETE_` prefix:

| Var | Purpose | Default |
|---|---|---|
| `KETE_HOST` | Proxy bind host | `127.0.0.1` |
| `KETE_PORT` | Proxy bind port | `8080` |
| `KETE_DB_PATH` | Override `~/.kete/memory.db` | (unset) |
| `KETE_PROJECT_PATH` | Force MCP project key | (unset; falls back to folder-name of `WORKSPACE_FOLDER_PATHS[0]` or `cwd`) |
| `KETE_UPSTREAM` | Default upstream selection | `anthropic` |
| `KETE_CC_PROXY_URL` | cc-proxy base URL | `http://127.0.0.1:8787` |
| `KETE_CC_PROXY_KEY` | cc-proxy `x-api-key` value | (required when upstream is `cc-proxy`) |
| `KETE_DRIFT_MODEL` | Override extraction model id | `claude-haiku-4-5-20251001` |
| `KETE_DRIFT_CHECK_INTERVAL` | Drift checks per N prompts | `5` |
| `KETE_TOKEN_WARNING_THRESHOLD` | Compaction warning at this token count | `160000` |
| `KETE_TOKEN_CLEAR_THRESHOLD` | Compaction clear at this token count | `180000` |
| `KETE_EXTENDED_CACHE` | Enable Anthropic prompt-cache keep-alive | `false` |
| `KETE_REQUEST_TIMEOUT` | Upstream forward timeout (ms) | `300000` |
| `KETE_BODY_LIMIT` | Inbound body size limit | `10485760` |

AWS env vars (`AWS_REGION`, `AWS_PROFILE`, etc.) are read by the AWS SDK directly when the upstream is `bedrock`. We do not wrap them in `KETE_AWS_*`; they're cross-tool conventions.

**HTTP headers.** kete uses `x-kete-*` for any headers it consumes or stamps:

| Header | Direction | Purpose |
|---|---|---|
| `x-kete-upstream` | inbound | Per-request upstream override (`anthropic` \| `cc-proxy` \| `bedrock`) |
| `x-kete-request-id` | inbound + outbound | Correlation id; if absent inbound, kete generates and stamps |

`x-kete-*` headers are stripped before forwarding to any upstream — they are kete-internal.

## Options considered

- **`~/.kete/` and `KETE_*`.** What we picked.
- **`~/.local/share/kete/` (XDG) and `KETE_*`.** XDG-correct on Linux, ignored on macOS, less greppable. Reject for v1; revisit if a real user complains.
- **`~/Library/Application Support/kete/` on macOS.** OS-idiomatic; not consistent with `~/.kete/` on Linux, which fragments docs. Reject.
- **`~/.grov/` and `GROV_*`.** Compat with grov-the-product. We are not grov; rejected by ADR 0000.

## Consequences

Easier:

- Greppability: `KETE_` prefix is unambiguous; no env var collision with grov, cc-proxy, or anything else dreamware-nz runs.
- Clear demarcation: `~/.kete/` contains only kete state; `~/.grov/` (if a user has both installed) belongs to grov.
- Documentation table in `docs/reference/env.md` is one block, no annotation about "which prefix in which version".

Harder:

- Existing users running kete builds before this ADR landed have to migrate (rename `~/.grov/` → `~/.kete/`, set new env vars). There are no such users — this ADR landed before any code did. No migration.
- AWS users hit a small inconsistency: most kete things are `KETE_*` but AWS creds are `AWS_*`. We accept the inconsistency in exchange for compat with the AWS SDK's existing chain. Documented in `docs/how-to/use-bedrock.md`.
