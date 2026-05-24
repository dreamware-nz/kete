---
number: 0009
title: Pin extraction model to claude-haiku-4-5-20251001 with KETE_DRIFT_MODEL override
date: 2026-05-24
status: accepted
brief: 011-reasoning-extraction
supersedes: null
superseded-by: null
---

# 0009 — Pin extraction model to claude-haiku-4-5-20251001 with KETE_DRIFT_MODEL override

## Context

Extraction (intent, summary, drift score, correction message, "should this update an existing memory" decision) is a Haiku call in the TS code: model id `claude-haiku-4-5-20251001`. Haiku is cheap, fast, and produces structured JSON the code parses. The model id is hard-coded in `src/core/extraction/llm-extractor.ts`.

Two reasons to keep it pinned:

1. **Wire compat with captured-task shape.** The extraction prompts produce JSON whose fields the TS code consumes. A different model produces different field-coverage and different field formats; downstream code breaks subtly.
2. **Cost predictability.** Haiku is cheap. A user paying their own API key has a stable mental model of what extraction costs.

One reason to allow override: the TS code already exposes `KETE_DRIFT_MODEL` as an env var. Operators experimenting with cheaper or smarter models should be able to do so without recompiling.

## Decision

kete pins the default extraction model to `claude-haiku-4-5-20251001`. The `KETE_DRIFT_MODEL` environment variable overrides the default. The model id is read once at startup; mid-run model changes are not supported.

We do not introduce per-call model selection (different model for drift than for summary). One model handles all extraction calls.

## Options considered

- **Pin Haiku, allow `KETE_DRIFT_MODEL` override.** What we picked. Matches TS exactly.
- **Auto-select based on the user's session model** (if user is using Sonnet, drift on Sonnet; if Opus, drift on Opus). Tempting because it shares one API key context, but it breaks cost predictability and changes the captured shape with the user's session. Reject.
- **Self-host a small model for extraction.** The Pareto-optimal answer at sufficient scale, but out of scope for this port. Reject.
- **Different models per call site.** More flexible, more configuration surface, no demonstrated win. Reject.

## Consequences

Easier:

- Tests can record fixtures against a known model id and stay valid for the lifetime of that model deprecation cycle.
- `kete doctor` can sanity-check model id format and cite the env var.
- A new Haiku version (5? 6?) is a one-line change.

Harder:

- When Anthropic deprecates `claude-haiku-4-5-20251001`, we have to ship a new release pinning a successor. Cite this ADR in the deprecation-bump ADR.
- If extraction quality bites a user, the only escape is `KETE_DRIFT_MODEL` to a more expensive model. That's the right escape — not "auto-fall-back to GPT-4 if Haiku flubs".
