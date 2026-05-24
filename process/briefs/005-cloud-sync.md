---
id: 005-cloud-sync
date: 2026-05-24
status: deferred
from-idea: 2026-05-24-kete
design: null
adrs: [0016-cloud-sync-deferred]
plan: null
---

# 005 — Cloud sync (deferred to a future brief)

## Status

**Deferred.** Per ADR 0000 and ADR 0016, kete v1 ships local-first with no cloud sync. This brief is preserved as a placeholder so the chain stays explicit.

## Why deferred

- **No second user yet.** "Team memory" is the value prop only when there are at least two developers. dreamware-nz is currently one. Building a sync layer for one user is speculative.
- **No backend to sync to.** The grov backend (`api.grov.dev`, Supabase) belongs to `TonyStef/Grov`. We can't use it without coupling kete to a third party's infrastructure. We'd need to build or pick our own.
- **Schema is still moving.** The captured-task shape will iterate during v1 against real dreamware-nz workflows. Locking it into a wire contract before it stabilises produces a brittle sync layer.
- **Local store covers the personal-cross-machine case.** A developer with two machines can sync `~/.kete/memory.db` via syncthing, rsync, a git repo of exported tasks, or whatever they already use for dotfiles. This is good enough until "team memory" is a real concern.

## What this brief covers when revived

When kete acquires a second user (or a clear team-memory need), this brief comes back to `draft` and is rewritten. The shape will look something like:

- A self-hosted backend, or a deliberate-vendor-pick (Supabase, NATS, plain Postgres + a tiny Go REST service). Not `api.grov.dev`.
- Authentication that fits dreamware-nz (probably not GitHub OAuth via Supabase, given the deferral wipes that decision).
- A sync protocol designed against the *then-current* `tasks` schema, not today's.
- Merge / supersede semantics for memories edited or evolved by a teammate.
- A cloud-side ranking story for `kete_preview`'s context retrieval.

## What stays out of v1

- `kete login` / `kete logout` subcommands.
- `kete sync` subcommand.
- `~/.kete/credentials.json`.
- `KETE_API_URL` env var.
- Any HTTP client code in kete that points outward beyond the model API forwarder.

## Doc impact

- `[none]` until this brief reactivates.
- When reactivated: how-to for login, reference for the cloud API we end up with, explanation of the sync protocol.

## Open questions

All deferred. The most useful artefact for the future-revival is honest documentation in `docs/explanation/why-no-cloud-yet.md` (small, optional), so the deferral is visible to users rather than silent.
