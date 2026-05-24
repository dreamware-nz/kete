# Why three upstreams

kete forwards `POST /v1/messages` to one of three upstreams,
chosen per request:

1. **anthropic-direct** — `https://api.anthropic.com`. Pay-per-token
   on your own API key.
2. **cc-proxy** — `dreamware-nz/cc-proxy`, the Swift menubar app
   that maps your Claude Code subscription to an Anthropic-shaped
   HTTP server on `127.0.0.1:8787`.
3. **bedrock** — AWS Bedrock's `bedrock-runtime` endpoint, with
   SigV4 signing and event-stream-to-SSE demuxing.

ADR 0015 captures the decision. This doc explains the *why*.

## Why not just one

The natural design is "let the user pick at startup, done." That
falls down because **dreamware-nz developers want to switch
mid-session without restarting the proxy.** Concretely:

- A subscription user usually wants `cc-proxy` (free under their
  Claude Code seat) but occasionally needs to fall through to
  direct API for a model the subscription doesn't expose.
- A team running on AWS wants `bedrock` for billing/compliance but
  drops to direct API for early-access models.
- A user testing kete itself wants to A/B between paths without
  re-launching every time.

Per-request selection is what the surface needs to support.

## The selection rule

ADR 0015's precedence:

1. **Header `x-kete-upstream`** wins if present. Values:
   `anthropic` | `cc-proxy` | `bedrock`. Consumed and stripped
   before the request hits the upstream.
2. **Model-id pattern** if no header. ARNs and inference profiles
   matching `arn:aws:bedrock:` / `us.anthropic.` /
   `anthropic.claude` route to Bedrock. Plain `claude-…` is
   ambiguous between Anthropic-direct and cc-proxy (same model
   ids), so it falls through.
3. **`KETE_UPSTREAM` env var** as the global default.
4. **`anthropic`** if nothing else matches.

`kete doctor` validates the env var so a typo (`KETE_UPSTREAM=cc_proxy`)
fails loudly at startup, not after a 401 mid-session.

## What "wire-identical" means in practice

cc-proxy is wire-identical to anthropic-direct: same body, same
SSE, same prompt-cache semantics. ADR 0006's byte-exact rule holds
for both. The cc-proxy adapter literally reuses the anthropic
adapter (`internal/adapter/ccproxy/ccproxy.go`).

Bedrock is *not* wire-identical. ADR 0014 lists three differences:

1. **Auth.** SigV4 per request. `aws-sdk-go-v2`'s signer.
2. **URL/body shape.** Path is `/model/{id}/invoke[-with-response-stream]`.
   Body has no `model` field, no `stream` field, and uses
   `anthropic_version: "bedrock-2023-05-31"` in the body instead
   of the `anthropic-version:` header.
3. **Response framing.** AWS event-stream binary frames vs SSE.
   We demux on the response side and re-emit Anthropic-shaped SSE
   events so clients dispatch on the same event names
   (`message_start`, `content_block_delta`, etc.) regardless of
   upstream.

## What the proxy doesn't do

- **No automatic fallback.** If cc-proxy is down, kete returns
  cc-proxy's error. You configure your client's fallback policy.
- **No request fanout.** One request goes to one upstream.
- **No model translation.** The model id in your body is the model
  id we forward (or, for Bedrock, the model id we use for the URL
  and strip from the body). We don't translate `claude-sonnet-4-5`
  into `us.anthropic.claude-sonnet-4-5-…` for you.
- **No `/v1/responses`.** That's OpenAI's shape; we don't proxy it
  for v1. (Brief 000 non-goals.)

## Why this matters for memory

Memory injection happens **before** upstream selection. The proxy
splices prior memory into the Anthropic-shaped body, then hands
that to the chosen adapter. cc-proxy and Anthropic-direct see the
injected body byte-exact; Bedrock sees the same body after the
ADR 0014 translation. Memory is wire-agnostic — verified live on
Bedrock with the "kowhai" round-trip.

## See also

- ADR 0015 — the decision.
- ADR 0014 — the Bedrock exception.
- ADR 0006 — byte-exact passthrough (applies to anthropic + cc-proxy).
- `docs/how-to/use-bedrock.md`
- `docs/how-to/use-cc-proxy.md`
- `docs/reference/proxy.md` — env vars and selection precedence.
