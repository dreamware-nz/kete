---
number: 0001
title: Ship as a single static Go binary, target Go 1.22+
date: 2026-05-24
status: accepted
brief: 000-go-port-overview
supersedes: null
superseded-by: null
---

# 0001 — Ship as a single static Go binary, target Go 1.22+

## Context

kete needs a distribution story. The user runs a single binary on their machine; they should not need a Node/Python/Ruby toolchain installed first, should not be subject to `postinstall` shenanigans, and should be able to upgrade by replacing one file. Go is the obvious fit: a self-contained, statically-linked binary per OS/arch that runs on a machine with nothing else installed.

Go 1.22 is the lowest version that gives us `slog` (structured logging in stdlib) and `for range over int`, both of which avoid third-party deps we'd otherwise pull in.

## Decision

We target Go 1.22 or later and ship `kete` as a single statically-linked binary, cross-compiled to `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`. Distribution is via GitHub Releases (binary downloads) and `go install github.com/dreamware-nz/kete/cmd/kete@latest`. No package manager beyond what each user already has.

## Options considered

- **Go 1.22+, static binary, GitHub Releases.** What we picked. Simplest distribution story; no runtime dependency; no postinstall.
- **Go 1.21.** No `slog` in stdlib (well, present, but ergonomics worse), no integer-range. Reject.
- **TypeScript / Node.** Forces a Node install and a `postinstall` story we don't want. Reject.
- **Rust.** Solves the same distribution problem. The cost/value gap to Go is small and the team has more Go reps. Reject.

## Consequences

Easier:

- One binary per release artefact. `kete doctor` can `os.Executable()` itself with confidence.
- No language-runtime toolchain on user machines.
- `--version` is `-ldflags "-X main.version=…"` — one source of truth.

Harder:

- We lose `npm install -g`'s familiarity. Need a clean `curl | sh` install.sh for the fast path.
- Cross-compiling requires the SQLite driver to be cgo-free (forces ADR 0002).
- No automatic post-install login. Users run authentication subcommands explicitly. Acceptable; cloud sync is deferred (ADR 0016) and there is nothing to log into yet.

We will revisit the Go version floor in a year if a feature in the stdlib (e.g. better HTTP/2 control, `iter` adoption) tempts us. Cross-compiling targets are a config knob, not an architectural commitment.
