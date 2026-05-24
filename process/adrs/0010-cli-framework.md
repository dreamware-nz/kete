---
number: 0010
title: CLI uses spf13/cobra
date: 2026-05-24
status: accepted
brief: 001-cli-shell
supersedes: null
superseded-by: null
---

# 0010 — CLI uses spf13/cobra

## Context

kete's CLI surface is small (about a dozen subcommands; brief 001) but has nested flags (`kete proxy --debug --extended-cache`), needs sensible `--help` output, and needs to be obvious enough that a new contributor can find a command's implementation without grepping the world. Two honest options in Go:

- **`spf13/cobra`** — the de-facto Go CLI framework. Widely understood. Generates `--help`, sub-command trees, and shell completions. Pulls `spf13/pflag` and a small dep graph.
- **stdlib `flag` plus a `switch` on `os.Args[1]`** — Pike-flavoured. Smaller. Honest. Forces us to write per-subcommand flag parsing by hand.

Pike's lineage says `flag` for tools this size. But:

- Nested subcommands plus "no subcommand → help" plus consistent flag inheritance costs ~150 LOC to reimplement honestly with `flag` alone.
- Every Go developer with five years' experience can read a cobra tree without thinking. Hand-rolled `flag` dispatch costs new contributors a half-hour of "where does this command live".
- Cobra is widely deployed inside cgo-free single-binary tools (gh, kubectl, hugo). Its dependency footprint is negligible at our binary size.

The honest answer is: cobra's flexibility is what we'd reinvent badly. Take it.

## Decision

kete uses `github.com/spf13/cobra`. The root command is `kete`. Each subcommand is a file under `cmd/kete/`, attached to the root in `main.go`. Subcommand `Run` functions take a `*cobra.Command` and `[]string`, return error, and rely on the top-level `main` to print the error and exit non-zero.

We do NOT use cobra's `Persistent*` hooks for DB lifecycle. The DB connection is opened lazily inside subcommands that need it; `defer store.Close()` in each. This avoids the "every `kete --help` opens the DB" trap.

## Options considered

- **`cobra`.** What we picked. Familiar; well-understood ergonomics.
- **stdlib `flag` + hand-rolled dispatch.** Simpler in raw terms; takes more LOC overall when you handle nested subcommands and help output. Reject.
- **`urfave/cli`.** Pleasant API, smaller community. We'd be picking it for taste over function. Reject.
- **`kong`** — declarative, struct-tag-based. Elegant for small tools; loses on the same nested-subcommand and persistent-flag complexity that pushed us off `flag`. Reject.

## Consequences

Easier:

- New subcommands are one new file plus an `AddCommand` line.
- Shell completions for free if we want them (we'll skip in v1; freebie when it matters).
- Familiar `--help` output.

Harder:

- We carry cobra's transitive deps. Trivial.
- Any pattern that wants to do something "before every command runs" has to resist `PersistentPreRunE`. We commit to: the place to do that is the function in `main.go` that constructs subcommands, not a hook.

If we ever need to ship a self-contained binary with truly minimal deps (TinyGo, embedded constraints), this ADR gets superseded. Not preempting.
