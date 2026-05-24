---
number: 0016
title: Cloud sync deferred to a future brief
date: 2026-05-24
status: accepted
brief: 005-cloud-sync
supersedes: null
superseded-by: null
---

# 0016 — Cloud sync deferred to a future brief

## Context

kete is a memory layer for the dreamware-nz workflow on Crush. "Team memory" — the value prop where one developer's captured reasoning seeds another developer's session — only matters when at least two developers run kete against a shared store. dreamware-nz is currently one developer. Building a sync layer for one user means designing a schema-on-the-wire, picking an auth story, choosing a backend, and writing a synchroniser — all speculative work paid up-front before the value materialises.

There is also no backend to sync to that we own. We could couple kete to a third-party backend, but that's an ongoing operational dependency on someone else's infrastructure. The local SQLite store at `~/.kete/memory.db` is enough for one user; "sync" across that user's machines can ride syncthing, dropbox, rsync, or whatever the user already uses for dotfiles.

## Decision

Cloud sync is **deferred** to a future brief. Brief 005 is rewritten with `status: deferred`.

Concretely, kete v1 ships:

- No `kete login` / `kete logout` / `kete sync` subcommands.
- No `~/.kete/credentials.json` file.
- No outbound HTTP from kete except (a) forwarding to the configured upstream, and (b) the Haiku extraction calls (which use the user's own API key via the same forwarder).
- No `KETE_API_URL` env var.

When kete acquires a real second user — or when team-memory becomes a concrete need rather than a hypothetical — brief 005 reactivates with `status: draft` and is rewritten against the *then-current* `tasks` schema, the *then-current* deployment options (self-hosted vs vendor pick), and the *then-current* auth story.

## Options considered

- **Defer cloud sync entirely.** What we picked.
- **Build sync to `api.grov.dev` anyway.** Couples kete to a third party's infrastructure indefinitely. Reject; ADR 0000 already removed this option.
- **Build sync to a self-hosted backend now.** Requires designing schema, auth, deployment, and operational surface for one user. Speculative cost paid up-front. Reject.
- **Build local file-based "sync" (e.g. export tasks as JSON, sync via syncthing).** Would work for the personal-cross-machine case, but invents a file format we'd then have to live with. The simpler answer is "syncthing on `~/.kete/memory.db` directly works fine for one user across two machines"; we don't need to invent anything.

## Consequences

Easier:

- Less code to ship in v1. No HTTP client beyond the upstream forwarder. No credential management. No retry/batching for sync.
- The captured-task schema can iterate during v1 without breaking a wire contract.
- The cloud-sync brief can be designed properly when we know who the team is and what they need, rather than guessing now.

Harder:

- Single-user only. A second dreamware-nz developer using kete today gets their own local store; their captures don't reach the first user. We accept this; it's the right time-to-ship trade-off.
- Personal cross-machine sync is the user's problem (syncthing, rsync, dropbox, whatever they already use for dotfiles). We don't own that surface. We document it briefly in a future how-to.
- The day cloud sync is actually wanted, we design and ship it then — against the schema and team shape that exist at that moment, not the ones we'd guess at today.
