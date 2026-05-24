---
id: 011-reasoning-extraction
date: 2026-05-24
status: accepted
from-idea: 2026-05-24-kete
design: null
adrs: [0009-haiku-as-extraction-model]
plan: 011-reasoning-extraction
---

# 011 — Reasoning extraction

## Problem

Captured tasks are only useful if the *reasoning* inside them is extractable. The TS implementation calls Claude Haiku (`claude-haiku-4-5-20251001`) at several points:

- `extractIntent(prompt)` — at session start, derive `goal`, `expected_scope`, `constraints`, `keywords` from the user's first message.
- `generateSessionSummary(steps)` — at session end (or auto-compaction), produce the structured summary used to write the `task` row.
- `analyzeTaskContext(task, prompt)` — when injecting prior memory, decide which captured tasks are most relevant (cloud has its own ranking; this is the local fallback / re-ranking).
- `shouldUpdateMemory(existing, new)` — when sync runs, decide whether a newly captured task supersedes an existing memory or stands alone.
- Drift scoring (brief 007) — same model, different prompts.
- Correction building (brief 007) — same model, generates the recovery message at intervene/halt levels.

Every Haiku call goes through `forwardToAnthropic()` (the same code the proxy uses to forward user requests), reusing `buildSafeHeaders()` so the user's API key is the one paying. Failure of an extraction call is non-fatal: a partial summary is better than no row.

kete implements each of these calls, with the same prompt strings (the prompts are an undocumented part of the contract), against the same model. Prompts live in `src/core/extraction/llm-extractor.ts` (1632 lines).

## Who is hurt

- Captured `tasks` rows whose `reasoning_trace`, `decisions`, and `constraints` are wrong or empty. The dashboard renders these directly.
- Drift detection, which depends on `extractIntent` returning a usable `expected_scope`.
- Anyone whose API key burns budget because extraction prompts retry on 5xx without backoff.

## Constraints

- Extraction model: `claude-haiku-4-5-20251001`. Override via `KETE_DRIFT_MODEL` (TS env var; reuse it).
- All extraction calls go through the proxy's Claude forwarder; reuse the same code path the user's traffic uses.
- Failure mode: log + continue. A failed extraction must not abort the surrounding operation.
- Prompts are part of the contract: they must produce the same structured output (parseable JSON) the TS prompts produce. A different prompt that produces "the same" output is a regression — capture-time and inject-time prompts have to round-trip.
- Cost discipline: extraction is amortised over many user prompts; per-call cost should be small (max-tokens caps in TS are deliberately tight). Match TS limits.
- Token accounting: extraction calls don't count against the user's session budget (different conversation), but they do hit the user's API key. `kete status` should surface a rough extraction cost in v2; not in v1.

## Success looks like

- Captured `tasks` rows on a fixture session contain a `goal`, `decisions` with rationale, and `files_touched` consistent with the actual session contents.
- A drift score on a fixture session is consistent across runs (deterministic prompts; same fixture &rarr; same score within LLM-nondeterminism tolerance). Already specified in brief 007.
- An extraction call that 5xx's once retries with exponential backoff and succeeds; a permanent 4xx surfaces as a structured log line and the surrounding write proceeds with empty extracted fields.

## Non-goals

- Replacing Haiku with a self-hosted or cheaper model. The model is part of the wire contract; changing it changes the captured-task shape.
- Caching extraction outputs. Each call is on demand.
- A prompt-engineering tooling layer. The prompts live in code; we read them from the TS file and port them.
- Streaming extraction responses. Extraction is synchronous; small responses; no streaming gain.

## Open questions

- `[adr]` 0009 — Pin to `claude-haiku-4-5-20251001` for v1, with `KETE_DRIFT_MODEL` override. When Anthropic deprecates or a cheaper Haiku ships, that's a new ADR.
- How exactly to port the prompts. Mechanical copy (`go:embed` the strings from a `prompts/` directory) is honest; reformatting them in Go code is a regression risk. Embed.
- Whether to surface a `--dry-run` extraction mode for `kete drift-test`. TS doesn't; consider for v1.5.

## Doc impact

- `docs/explanation/extraction.md` `[new]` — what the model is asked to do at each call site.
- `docs/reference/env.md` `[update]` — `KETE_DRIFT_MODEL`.
- Prompts themselves live in source (`internal/extraction/prompts/*.txt`); not user-facing.
