---
number: 0014
title: Bedrock vendor uses AWS SDK for SigV4 + event-stream-to-SSE conversion
date: 2026-05-24
status: accepted
brief: 012-bedrock-vendor
supersedes: null
superseded-by: null
---

# 0014 — Bedrock vendor uses AWS SDK for SigV4 + event-stream-to-SSE conversion

## Context

Bedrock's Anthropic-models endpoint differs from `api.anthropic.com` on three axes that matter to the proxy:

1. **Auth.** Each request is signed with AWS SigV4 over the canonical request (method, path, query, headers, body hash). The signature includes the host and timestamp; we cannot pre-sign once at startup.
2. **URL / body shape.** Path is `/model/{modelId}/invoke` (or `…/invoke-with-response-stream`). Body has no `model` field (it's in the URL) and uses `anthropic_version: "bedrock-2023-05-31"` instead of `anthropic-version: …` header.
3. **Streaming format.** Responses use AWS event-stream — binary frames with `:event-type` headers and CRC32s — not the SSE that Claude Code consumes. We must demux event-stream frames and re-emit them as SSE on the wire to the client.

ADR 0006 says we never re-marshal request bodies in the hot path. Bedrock forces an exception: we *must* re-shape the body (drop `model`, swap version field, possibly remap `model` → URL `modelId`). We accept losing direct-API cache hits on Bedrock requests; Bedrock has its own prompt cache anyway and the keying is its own.

Hand-rolling SigV4 is well-understood territory but easy to get subtly wrong (canonical-header sorting, `x-amz-content-sha256` for streaming bodies, double-encoded path segments). The AWS SDK for Go v2 ships a battle-tested `aws/signer/v4` package and a credential chain we get for free.

## Decision

The Bedrock adapter:

- **Signing** uses `github.com/aws/aws-sdk-go-v2/aws/signer/v4`. We sign per request, immediately before forwarding, with credentials resolved via the standard SDK chain (env, shared profile, SSO, IRSA, instance profile) at proxy startup.
- **Body translation** happens in the adapter's `Wire.Forward`:
  - Strip `model` from the body, store on the request context.
  - Replace `anthropic-version` header with `anthropic_version` body field set to `bedrock-2023-05-31` if not already set.
  - Compute upstream URL: `https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke-with-response-stream` for streaming requests, `…/invoke` otherwise.
  - Re-marshal the body. We accept the byte-level change.
- **Response streaming** uses `github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream` to demux frames. Each `chunk` event's payload is JSON; we wrap it in SSE format (`event: <type>\ndata: <json>\n\n`) matching the shape Claude Code expects from Anthropic-direct.
- **Error mapping** translates Bedrock error responses (`ValidationException`, `ThrottlingException`, etc.) into Anthropic-shaped error JSON so client error handling doesn't have to know it's Bedrock.

The adapter implements `Wire` and `Semantics` from ADR 0007. `Semantics` is shared with the Anthropic-direct adapter (same body fields once the wire-level adjustments are done).

## Options considered

- **AWS SDK + event-stream package.** What we picked.
- **Hand-rolled SigV4.** ~400 lines, easy to get wrong on edge cases (URL encoding, streaming body hashes, session-token handling). Reject.
- **Pre-sign requests at startup.** Doesn't work — signatures are time-bound (15-minute window) and per-canonical-request.
- **Forward without conversion (let client speak event-stream).** Claude Code doesn't speak event-stream. Reject.
- **Run Bedrock as a separate sidecar process.** Operational overhead for no benefit. Reject.

## Consequences

Easier:

- AWS credential resolution is one line of SDK config; we get env / shared file / SSO / IRSA / instance profile for free.
- The signing code is tested by Amazon. Our exposure is the adapter's call site, not the crypto.
- Event-stream demux is a maintained library; new event types from Bedrock land without our changing parsing.

Harder:

- One AWS SDK pulls a non-trivial dep graph. The binary grows by a few MB. Acceptable.
- Bedrock requests don't share Anthropic's prompt cache. We document the cost gap; users who care can run two proxies (one per upstream).
- Body re-marshalling is a foot-gun. We do it in exactly one place (`bedrock.Wire.Forward`); ADR 0006's rule is reaffirmed for every other code path. Code review should reject any other re-marshal.
- Streaming format conversion is per-request work. Negligible CPU; mentioned for honesty.
- Error shape mapping is best-effort. A bizarre Bedrock-specific error may surface as a synthetic Anthropic 500. We log the original; the user sees a useful message.
