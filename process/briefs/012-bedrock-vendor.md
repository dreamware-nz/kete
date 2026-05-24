---
id: 012-bedrock-vendor
date: 2026-05-24
status: shipped
from-idea: 2026-05-24-bedrock-support
design: null
adrs: [0014-bedrock-sigv4-and-event-stream, 0015-three-upstreams-selection]
plan: 012-bedrock-vendor
---

# 012 — Bedrock as a third vendor

## Problem

A non-trivial fraction of users run Claude through AWS Bedrock rather than `api.anthropic.com` — for compliance, billing consolidation, VPC routing, or because their org already has an AWS contract. The TS implementation forwards exclusively to `api.anthropic.com`; Bedrock users today have no kete option: they would have to point Crush at Bedrock directly, with no capture, no injection, no drift detection.

The differences between the two upstreams are small at the message level (Bedrock's body is Anthropic's body minus a couple of fields) and substantial at the wire level (path templating, SigV4 auth signing, AWS event-stream framing on responses). We cannot pretend Bedrock is "Anthropic with a different base URL".

## Who is hurt

- AWS-shop developers who can't use the direct API for policy reasons. Today they have no grov.
- Mixed teams where some devs are on direct-API and some on Bedrock. Captured memories live in different places, sync gets ugly.
- Anyone who wanted to evaluate Bedrock vs direct on cost without losing their captured reasoning.

## Constraints

- **Wire compat with both upstreams** at the same time. One proxy instance must be able to forward to either based on the routing rules below.
- **Anthropic-direct path stays default.** Existing users see no behaviour change.
- **AWS SDK for Go v2** for SigV4 signing and credential resolution. We do not hand-roll SigV4.
- **Body translation is a re-serialisation.** This is the only place in the proxy that re-marshals JSON. ADR 0006's byte-exact rule still holds for Anthropic-direct; Bedrock requests pay the cache penalty by definition (the upstream has its own cache anyway).
- **Streaming format conversion.** Claude Code expects SSE on the way back; Bedrock returns AWS event-stream (binary frames with CRCs). We convert.
- **No new HTTP route.** Both upstreams share `POST /v1/messages`. Routing is internal.
- **Credentials never logged.** SigV4 signed headers and the AWS access key both fall under `SENSITIVE_HEADERS`.
- **Models named consistently.** A captured `task.system_name` is the *direct-API* model id (`claude-sonnet-4-20250514`) regardless of whether the request was forwarded via Bedrock. This keeps team-shared memories portable.

## Success looks like

- A user with valid AWS creds and `KETE_UPSTREAM=bedrock` runs `kete proxy`, points Claude Code at it (`ANTHROPIC_BASE_URL=http://127.0.0.1:8080`), and a session works end-to-end: request goes to Bedrock, response streams back, a `tasks` row lands in `~/.kete/memory.db` with the same shape as a direct-API session.
- A user with both `ANTHROPIC_API_KEY` and `AWS_REGION` set, default `KETE_UPSTREAM` unset, gets Anthropic-direct (no surprise for existing users).
- Sending `x-kete-upstream: bedrock` with `KETE_UPSTREAM=anthropic` overrides for that one request.
- A request whose `model` field is `arn:aws:bedrock:us-west-2:…:inference-profile/…` is routed to Bedrock automatically, even with `KETE_UPSTREAM=anthropic`.
- `kete doctor` reports both upstreams' reachability when both are configured.

## Non-goals

- Bedrock for OpenAI-compatible models. OpenAI Codex via Bedrock is not a thing today; if it ever is, separate brief.
- Bedrock-only features (cross-region inference profiles beyond what the model id encodes, Bedrock guardrails, Bedrock knowledge bases). Out of scope.
- Replacing the AWS SDK with hand-rolled SigV4. Tempting; not worth the risk.
- A Bedrock-specific cache strategy. Bedrock has its own prompt-cache behaviour; we forward and trust it.
- Cross-account / `AssumeRole` flows beyond what the standard SDK credential chain handles.

## Open questions

- `[adr]` 0014 — SigV4 signing strategy, event-stream → SSE translation, body re-marshalling shape. Settled.
- `[adr]` 0015 — Upstream selection rules (header > model-id pattern > env var) and the default. Settled.
- Whether to record the upstream in `tasks` (`source: "bedrock"`). Probably yes; one column added per ADR 0003. Defer to plan stage.
- Whether `KETE_BEDROCK_MODEL_MAP` is needed in v1 or can wait. If most users put Bedrock model ids in their client config directly, no remapping needed. Ship without; add if asked.
- Caching: Bedrock's prompt cache is real and its keying is its own. We don't try to share cache state across upstreams.

## Doc impact

- `docs/how-to/use-bedrock.md` `[new]` — env vars, AWS credential setup, model-id forms.
- `docs/reference/env.md` `[update]` — `KETE_UPSTREAM`, `AWS_REGION`, `KETE_BEDROCK_MODEL_MAP`.
- `docs/reference/proxy.md` `[update]` — routing rules.
- `docs/explanation/two-upstreams.md` `[new]` — why Anthropic-direct preserves cache and Bedrock doesn't, what that means for cost.
- `README.md` `[update]` — quickstart paragraph for Bedrock.
