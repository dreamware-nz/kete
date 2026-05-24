---
id: NNN-slug
date: YYYY-MM-DD
status: draft         # draft | active | done | abandoned
brief: NNN-slug
design: null          # NNN-slug if this plan implements a design doc
adrs: [NNNN-foo, NNNN-bar]
---

# {{Title}}

## Goal

One sentence. What "done" means for the whole plan.

## Phases

Each phase ships value standalone *and* fits in a fresh session's context (≤ ~5 source files touched, ≤ ~7 Context entries, single concern). If a phase only matters when the next one lands, merge them. If a phase needs the whole repo to execute, split it.

### Phase 1 — {{name}}

- **Outcome:** what's true when this phase is done.
- **Slice:** the smallest thing that delivers that outcome.
- **Context:** files, docs, ADRs the executor must read. ≤ ~7 entries.
- **Depends-on:** `[]` for independent; omit for default `[<previous phase>]`; explicit list for partial deps.
- **Done when:** observable signal(s). Include doc-impact items from the brief if user-visible.
- **Risks:** what could break or get ugly.

### Phase 2 — {{name}}

(same shape)

## Out of scope

What this plan deliberately does not do. Cross-reference the brief's non-goals; add anything that emerged during planning.

## Assumptions

Things this plan rests on. If one of these breaks during execution, the plan stops and goes back to brief or ADR.
