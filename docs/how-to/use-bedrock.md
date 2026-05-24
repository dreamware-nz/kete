# How-to: use AWS Bedrock as the upstream

Run kete against Anthropic models hosted on Bedrock. Live-verified
against `us.anthropic.claude-haiku-4-5-20251001-v1:0` on `us-west-2`.

## What you need

- AWS credentials in the standard SDK chain (env vars,
  `~/.aws/credentials`, SSO, IRSA, or instance profile). kete never
  reads them directly — `aws-sdk-go-v2` does.
- A region with Bedrock and the model(s) you want enabled.
- For Anthropic models past 3.5 Haiku, an inference profile id
  (e.g. `us.anthropic.claude-haiku-4-5-20251001-v1:0`); raw model
  ids (`anthropic.claude-haiku-4-5-20251001-v1:0`) are rejected
  with `Invocation … with on-demand throughput isn't supported`.

## Configure

```sh
export AWS_REGION=us-west-2          # or wherever you have access
export KETE_UPSTREAM=bedrock
./bin/kete proxy
```

Point your client at `http://127.0.0.1:8080` and use the inference
profile id as the model name in the request body:

```json
{
  "model": "us.anthropic.claude-haiku-4-5-20251001-v1:0",
  "max_tokens": 256,
  "messages": [{"role": "user", "content": "hi"}]
}
```

## What kete does

The Bedrock adapter (ADR 0014) does three things the
Anthropic-direct adapter doesn't:

1. **SigV4-signs every request.** Per-request, immediately before
   forwarding. Credentials resolve at proxy startup via the SDK chain.
2. **Translates the body.** Strips `model`, drops `stream` (the
   choice lives in the URL), sets `anthropic_version:
   "bedrock-2023-05-31"`. This is the deliberate ADR 0006 exception
   — Bedrock has its own cache; the Anthropic prompt-cache prefix
   doesn't apply.
3. **Demuxes the response.** AWS event-stream frames become SSE
   events with the proper Anthropic event names (`message_start`,
   `content_block_delta`, etc.) — wire-identical to direct API.

## Errors

Bedrock errors come back wrapped in Anthropic-shaped error JSON so
your client doesn't have to know it's Bedrock:

```json
{
  "type": "error",
  "error": {
    "type": "ValidationException",
    "message": "Invocation … with on-demand throughput isn't supported. Retry with the ID or ARN of an inference profile that contains this model."
  }
}
```

## Memory injection on Bedrock

Memory injection is wire-agnostic — it splices into the Anthropic
body shape *before* Bedrock translation. Verified live: a seeded
prior task containing the word "kowhai" was retrieved by Haiku 4.5
through the proxy on the next prompt. Input tokens 22 → 90 confirmed
the splice was real.

## Drift on Bedrock

Drift extraction goes Anthropic-direct by default (ADR 0009). To run
drift through your local proxy on Bedrock instead, point the
extractor at the proxy:

```sh
export KETE_ANTHROPIC_URL=http://127.0.0.1:8080
export KETE_DRIFT_MODEL=us.anthropic.claude-haiku-4-5-20251001-v1:0
./bin/kete drift-test --fixture testdata/drift/fixtures.json
```

`KETE_ANTHROPIC_URL` makes `extract.NewClient` skip the
ANTHROPIC_API_KEY check; auth becomes the proxy's problem (it
SigV4-signs).

## Limits

- Streaming and non-streaming both work.
- Capture, memory injection, drift detection, compaction, and the
  expand-loop orchestrator all run identically against Bedrock.
- Extended-cache keepalive (`--extended-cache`) is opt-in and only
  hits the Anthropic-direct upstream slot today; Bedrock keepalive
  is its own brief.
