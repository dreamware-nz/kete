---
number: NNNN
title: short imperative
date: YYYY-MM-DD
status: proposed      # proposed | accepted | superseded | deprecated
brief: NNN-slug
supersedes: null      # NNNN-other-adr or null
superseded-by: null   # filled in if and when this ADR is replaced
establishes-invariants: []  # filled in §6a after invariants are captured
modifies-invariants: []     # filled in §6a if rolling forward an existing invariant
---

# {{NNNN}} — {{Title}}

## Context

What forces are in play. What we know, what we don't, what we tried to defer but can't any longer (per Poppendieck — if you can still defer, do).

## Decision

The decision, in one or two sentences, present tense, active voice. "We use X."

## Options considered

- **X** — what we picked. Why.
- **Y** — why not.
- **Z** — why not.

If there were no real alternatives, you didn't need an ADR.

## Consequences

What becomes easier. What becomes harder. What we will have to revisit if conditions change. Be honest about the second category.

## Invariants

Properties this decision implies must always hold. State each in one sentence here; each gets captured as `process/invariants/NNNN-slug.md` after Accept (run `process-invariant`). If the decision establishes none, write "None" — but most real decisions establish at least one. If the decision rolls forward an existing invariant, name it as `modifies NNNN-slug` instead of writing a fresh sentence.

## Notes

Anything else. Links to captures, prototypes, conversations. Optional.
