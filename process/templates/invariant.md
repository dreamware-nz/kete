---
number: NNNN
slug: short-kebab-case
date: YYYY-MM-DD
status: active           # active | superseded | retired
introduced-by: NNNN-adr  # NNNN-adr | discovery | initial
supersedes: null         # NNNN-other-invariant or null
superseded-by: null      # filled in if and when this invariant is rolled forward
related-adrs: []         # ADRs that reference, depend on, or interact with this invariant
---

# {{NNNN}} — {{Statement, one sentence}}

## Statement

One sentence, present tense, falsifiable. "X always holds." If you can't state it in one sentence, it isn't an invariant — it's a design.

## Scope

Where this property must hold. Name the files, packages, types, layers, or wire boundaries. Tight enough that a reader can verify by inspection.

## How to verify

A specific, executable check. One of:

- **Type / construction** — a compile-time guarantee. Name the type and the constructor that makes the bad state unrepresentable.
- **Test** — a named test or assertion. `go test -run TestX ./internal/foo`.
- **Tool** — a linter, vet rule, or shell command. `gofmt -l .` empty, `grep -rn 'json.Marshal' internal/proxy/forward.go` empty.
- **Inspection** — a one-line check a human can run quickly when the others are impractical.

If you can't write a verification check, this isn't a real invariant — drop it or sharpen the statement until you can.

## Notes

What this implies, what it does *not* imply, and where the deliberate exceptions live (each exception should reference an ADR). Optional.
