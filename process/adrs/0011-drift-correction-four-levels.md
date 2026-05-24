---
number: 0011
title: Keep four drift correction levels at the same score thresholds
date: 2026-05-24
status: accepted
brief: 007-anti-drift
supersedes: null
superseded-by: null
---

# 0011 — Keep four drift correction levels at the same score thresholds

## Context

The TS implementation has four correction levels — `nudge`, `correct`, `intervene`, `halt` — keyed off a 1–10 alignment score:

| Score | Level     | Behaviour                                                  |
| ----- | --------- | ---------------------------------------------------------- |
| 8–10  | none      | No correction injected                                     |
| 7     | nudge     | Brief reminder of goal and scope                           |
| 5–6   | correct   | Full correction with recovery steps                        |
| 3–4   | intervene | Strong correction; verification required                   |
| 1–2   | halt      | Critical stop; mandatory first action specified            |

Plus an escalation discipline: each correction increments a per-session counter, each clean turn (score ≥ 8) decrements. At escalation ≥ 3, "forced recovery" mode generates a Haiku-built recovery prompt.

These thresholds and labels are baked into:

- Captured `tasks.tags` (`needs-review`, `had-drift`, etc.).
- The `steps.correction_level` column's CHECK constraint (`'nudge' | 'correct' | 'intervene' | 'halt'`).
- The `drift_log` audit table.
- The TS `correction-builder-proxy.ts` prompts (different prose per level).

Changing the level structure requires changes in all of those places *and* invalidates correlations with existing captured data. The brief recommends re-affirming.

## Decision

kete keeps the four levels with the same names, the same score thresholds, the same escalation/recovery rules, and the same forced-recovery trigger at escalation ≥ 3. The Haiku prompts that produce per-level correction text are ported verbatim (per ADR 0009 and brief 011).

## Options considered

- **Keep four levels, same thresholds.** What we picked.
- **Three levels** (drop `nudge`, fold into `correct`). Simpler; the TS `nudge` is reportedly low-signal in practice. But: schema change, doc change, captured-data correlation break. Save for a deliberate follow-up brief if data shows nudge is dead weight. Reject for v1.
- **Continuous correction strength** (interpolate between levels). Tempting on paper; loses the categorical handles `tags`, dashboards, and audit need. Reject.
- **Drop drift entirely until there's evidence it earns its keep.** Would simplify the proxy a lot. But the feature is shipped, in production, and there's a `kete drift-test` debug surface — too late to remove without a separate, evidence-driven brief. Reject.

## Consequences

Easier:

- The four levels and their thresholds are real categorical handles for the audit trail (`correction_level` column on `steps`, tag values on `tasks`, dashboard groupings).
- Eval / regression tests against captured fixtures can compare apples-to-apples across kete releases.

Harder:

- Any honest observation that the four-level system has a tuning problem (e.g. `intervene` and `halt` rarely diverge in practice) is now an ADR-shaped move, not a quiet refactor.
- The `forced_recovery` mode's threshold (escalation ≥ 3) is part of this decision; revisiting it requires this ADR's supersession or an explicit amendment ADR.
