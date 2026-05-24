---
number: 0015
title: Three upstreams (anthropic-direct | cc-proxy | bedrock); selection by header > model-id pattern > KETE_UPSTREAM
date: 2026-05-24
status: accepted
brief: 013-cc-proxy-upstream
supersedes: null
superseded-by: null
---

# 0015 — Three upstreams; selection by header > model-id pattern > KETE_UPSTREAM

## Context

Brief 013 adds `cc-proxy` as a third upstream alongside Anthropic-direct (the historical default) and Bedrock (ADR 0014). Anthropic-direct and cc-proxy are wire-identical from kete's perspective — same body, same SSE, same prompt-cache behaviour. The only differences are base URL and auth header. Bedrock is the one that needs body translation, SigV4 signing, and event-stream-to-SSE conversion (ADR 0014).

With three values, we need a deterministic precedence rule. The constraints:

- **Existing direct-API behaviour stays the default.** No header, no env, vanilla request → Anthropic-direct.
- **Per-request override is occasionally needed.** Debug, A/B, "this one model is only on Bedrock". Don't force a proxy restart.
- **Auto-detection by model id** can distinguish Bedrock from Anthropic-flavoured (by ARN/prefix), but cannot distinguish anthropic-direct from cc-proxy (same model ids). cc-proxy must therefore be selected by header or env, never by auto-detection.

## Decision

Route per request using this precedence (highest first):

1. **`x-kete-upstream` request header.** Values: `anthropic` | `cc-proxy` | `bedrock`. Stripped before forwarding.
2. **Model-id pattern match** on the request body's `model` field. Routes to `bedrock` if the value:
   - matches `arn:aws:bedrock:` (inference-profile ARN), or
   - matches `^([a-z]{2}\.)?(anthropic|amazon|meta|mistral|cohere|ai21)\.` (Bedrock model id, optional cross-region prefix).
   Otherwise falls through. The pattern never selects `cc-proxy` — that's reserved for explicit selection only.
3. **`KETE_UPSTREAM` environment variable** at proxy startup. Values: `anthropic` | `cc-proxy` | `bedrock`. Default `anthropic`.

Selection is recorded on the request context and used by the adapter dispatcher. Anthropic-direct and cc-proxy share the Anthropic adapter (ADR 0007's `Wire` + `Semantics` split); the dispatcher swaps base URL and auth header based on selection. Bedrock uses its own adapter (ADR 0014).

Failure modes:

- `cc-proxy` selected, cc-api unreachable → 502 with the underlying connect error.
- `bedrock` selected, no AWS credentials or `AWS_REGION` unset → 502 with a clear configuration error.
- `anthropic` selected with no `x-api-key`/`authorization` on the inbound request → 401 (matches today's behaviour; the upstream rejects it).

We do not silently fall back across upstreams. A misrouted request fails loudly so the user fixes the configuration rather than getting surprising bills against the wrong account.

## Options considered

- **Header > model-id > env, three values.** What we picked.
- **Auto-detect cc-proxy** (e.g. by inbound `x-api-key` matching the cc-api key shape). Magical; invites surprises. Reject.
- **Multiple grov instances, one per upstream.** A valid escape hatch and we mention it in docs, but a single proxy serving all three is the v1 goal.
- **Config-file routing rules.** Overkill for three upstreams. Revisit if the count ever grows past five or routing depends on payload contents beyond the model id.

## Consequences

Easier:

- Existing direct-API users: unchanged. `KETE_UPSTREAM` defaults to `anthropic`.
- Subscription users (`cc-proxy`): set `KETE_UPSTREAM=cc-proxy` and `KETE_CC_PROXY_KEY` and they're done.
- AWS users (`bedrock`): `KETE_UPSTREAM=bedrock` and `AWS_REGION` plus credentials in the standard chain.
- Mixed: per-provider header in the Crush config (or any client that supports custom headers) selects per-call without a proxy restart.

Harder:

- Three values to keep documented in sync across env reference, how-tos, and `kete doctor`. Tractable.
- The header is the only way a request body with a vanilla Anthropic model id reaches cc-proxy or vice-versa. Documented; the model-id heuristic is honest about not solving this case.
- A user who *meant* to be on `cc-proxy` but typoed `cc_proxy` gets a 4xx. We accept loud failure over silent fallback. `kete doctor` validates the env value.

If a fourth upstream ever lands (a cloud-managed Anthropic-shaped proxy from a competitor, say), this ADR gets superseded; the precedence chain stays.
