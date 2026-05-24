---
number: 0002
title: Use modernc.org/sqlite (pure Go, no cgo) for the local memory store
date: 2026-05-24
status: accepted
brief: 004-memory-store
supersedes: null
superseded-by: null
---

# 0002 — Use modernc.org/sqlite (pure Go, no cgo) for the local memory store

## Context

The memory store at `~/.kete/memory.db` is the only durable state grov has (brief 004). The TS port uses `better-sqlite3`, which is native and synchronous. The Go ecosystem has two practical SQLite drivers:

- `github.com/mattn/go-sqlite3` — cgo binding to upstream sqlite3.c. Fast (within single-digit % of C). Forces cgo, which kills `CGO_ENABLED=0` static cross-compilation.
- `modernc.org/sqlite` — pure-Go transpilation of sqlite3.c via the modernc toolchain. No cgo. Slower (10–30 % depending on workload), larger binary, transpilation occasionally lags upstream sqlite by a release.

Our distribution constraint (ADR 0001) is one static binary per OS/arch. cgo turns "cross-compile from a Mac to linux/amd64" into "spin up a linux/amd64 build host with a C toolchain". For a tool whose hot path is an HTTP proxy, not a database engine, the perf gap is uninteresting.

## Decision

Use `modernc.org/sqlite`. We accept slower SQLite operations and a larger binary in exchange for cgo-free static cross-compilation.

## Options considered

- **`modernc.org/sqlite`.** What we picked. Pure Go; cross-compiles without a toolchain; matches our distribution model. Trades raw perf for portability.
- **`mattn/go-sqlite3`.** Faster but cgo-only. Defeats ADR 0001's static-binary goal. Reject for the default; keep in pocket as a build tag if a perf regression ever forces our hand.
- **`crawshaw.io/sqlite`.** Fast, supports `unlock_notify`, but cgo and the dependency story is less stable. Reject.
- **A non-SQLite store** (BoltDB, BadgerDB, embedded RocksDB). Would require schema rewrite, would break wire-compat with the TS DB at `~/.kete/memory.db`, would ignore brief 004's lineage constraint. Reject.

## Consequences

Easier:

- `GOOS=linux GOARCH=arm64 go build` from a Mac just works.
- One Go module graph, no external `.so`/`.a` artefacts.
- Tests are hermetic — no system sqlite version to chase.

Harder:

- Per-query overhead is higher. For grov's workload (a few hundred reads/writes per session) it's not measurable; for any future workload that streams steps at high rate, we'd have to revisit.
- `modernc.org/sqlite` lags upstream sqlite by a release or two. We don't depend on bleeding-edge SQLite features (no JSON path operators, no the-newest-virtual-tables); this is fine.
- Binary size is ~5 MB larger than mattn's. Negligible.

If a real perf problem ever shows up under profiler, the path back is a build tag selecting `mattn/go-sqlite3` for users who can install a C toolchain. We will not invent that machinery preemptively.
