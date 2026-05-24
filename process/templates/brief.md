---
id: NNN-slug
date: YYYY-MM-DD
status: draft         # draft | accepted | rejected | shipped
from-idea: YYYY-MM-DD-slug
design: null          # filled if a design doc is drafted
adrs: []              # filled as ADRs are accepted
invariants: []        # filled as invariants are captured (usually via the ADRs)
plan: null            # filled when a plan is drafted
---

# {{Title}}

## Problem

What is broken or missing. Concrete. One short paragraph.

## Who is hurt

Real users or systems, not personas. If you can't name them, the brief isn't ready.

## Constraints

Things that are not negotiable. Time, compatibility, platforms, dependencies.

## Success looks like

Observable. "X works in Y under Z conditions." Not metrics theatre — one or two real signals.

## Non-goals

What this brief explicitly does *not* try to solve. Cuts scope before scope cuts you.

## Open questions

Things that will need an ADR, a design pass, an invariant captured, or some combination. Listing them here is enough; the ADRs / design / invariants come later. Tag each as `[adr]`, `[design]`, `[invariant]`, or any combination, so the next stage knows where it goes.

## Doc impact

What user-facing or contributor-facing docs this brief changes when it ships. Tag each item `[update]`, `[new]`, or `[none]`. `[none]` is a legitimate answer — an internal refactor with no behaviour change probably has no doc impact. The point is to *ask*.

Examples:
- `README.md` `[update]` — feature mention + flag table.
- `docs/tutorials/diff-two-dbs.md` `[new]` — first-run walkthrough.
- `docs/reference/cli.md` `[update]` — new flag.
- `CHANGELOG.md` `[update]` — under "Unreleased".
- `[none]` — internal refactor, no user-visible behaviour.
