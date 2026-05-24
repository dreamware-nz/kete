# Proxy reference

> Local HTTP proxy that captures Crush sessions, injects prior memory,
> and forwards byte-exact to one of three Anthropic-shaped upstreams.

Run with:

```sh
kete proxy
```

Configure via environment variables (no config file).

## Endpoints

| Method | Path           | Behaviour                                |
| ------ | -------------- | ---------------------------------------- |
| GET    | `/health`      | `200 {"status":"ok"}`                    |
| POST   | `/v1/messages` | Forward to upstream, capture, inject     |
| `*`    | `/*`           | `404 {"error":"Not found"}`              |

## Configuration

| Env var                       | Default              | Effect                                                         |
| ----------------------------- | -------------------- | -------------------------------------------------------------- |
| `KETE_HOST`                   | `127.0.0.1`          | Bind host                                                      |
| `KETE_PORT`                   | `8080`               | Bind port                                                      |
| `KETE_UPSTREAM`               | `anthropic`          | `anthropic` \| `cc-proxy` \| `bedrock`                         |
| `KETE_PROJECT`                | cwd                  | Project key for capture/inject; symlinks resolved              |
| `KETE_ANTHROPIC_URL`          | `https://api.anthropic.com` | Override the Anthropic-direct upstream                  |
| `KETE_DRIFT_CHECK_INTERVAL`   | `5`                  | Fire drift check every Nth request                             |
| `KETE_COMPACT_WARN_TOKENS`    | `160000`             | PreCompute fires at this usage                                 |
| `KETE_COMPACT_CLEAR_TOKENS`   | `180000`             | Apply fires at this usage                                      |

## Limits

- Body limit: **10 MB**. Larger requests get `413`.
- Request timeout: **5 minutes**. Slow upstreams cancel cleanly.
- Graceful shutdown: SIGINT/SIGTERM triggers `Shutdown` with a 500 ms
  budget; in-flight captures are awaited before exit.

## Header handling

Forwarded to the upstream (whitelist):

- `x-api-key`
- `authorization`
- `anthropic-version`
- `content-type`
- `anthropic-beta`

Anything else is dropped before forwarding. `x-kete-upstream` is
consumed (used for routing) and never forwarded.

Logged-but-redacted to `[REDACTED]`:

- `x-api-key`
- `authorization`
- `x-amz-security-token`, `x-amz-date`, `x-amz-content-sha256`

## Upstream selection (ADR 0015)

Per-request precedence:

1. `x-kete-upstream` header (`anthropic` | `cc-proxy` | `bedrock`)
2. Model-id pattern (`arn:aws:bedrock:…`, `us.anthropic.…`,
   `anthropic.claude…` → `bedrock`; ambiguous between Anthropic-direct
   and cc-proxy → fall through)
3. `KETE_UPSTREAM` env var
4. Default `anthropic`

cc-proxy and Bedrock land in plans 013 and 012; until then those
upstreams return `501 Not Implemented`.

## Byte-exact discipline (ADR 0006)

The proxy never re-serialises a parsed JSON body in the forward path.
Memory injection and (future) drift correction operate on the raw
`[]byte` via `internal/inject`, validating the result still parses
before forwarding. This is what keeps Anthropic's prompt-cache prefix
matching working — see
`docs/explanation/raw-body-preservation.md`.

## Capture

After every successful forward, an async goroutine writes a `tasks`
row at:

- `id`: UUIDv4
- `project_path`: resolved cwd or `KETE_PROJECT`
- `source`: `proxy`
- `reasoning_trace`: the **pre-injection** request body (so the
  captured trace doesn't include kete's own splices)

The same goroutine then runs `extract.ExtractTask` against the body
and updates the row with `goal`, `decisions`, and `files_touched`.
Bounded to 60s; failures are logged and swallowed — the raw row
stays.

## Drift detection

Every Nth request (`KETE_DRIFT_CHECK_INTERVAL`, default 5) the proxy
fires a background `drift.ScoreAction` against the most recent
enriched task's goal. Score < 8 yields a level (none / nudge /
correct / intervene / halt; ADR 0011), and a `drift.BuildCorrection`
call drafts a correction message that's queued for the **next**
request — `injectCorrectionPayload` splices it before the cache
breakpoint on the way out. Score >= 5 lands in `steps`; < 5 lands in
`drift_log` with the correction text.

Per-session escalation counter increments on each correction,
decrements on recovery (LevelNone), never goes below zero. Forced
recovery threshold at escalation >= 3.

## Memory injection

Before forwarding, the proxy looks up `ListTasks(project)`, picks
the top 3 newest, renders them via `inject.Preview`, and splices the
result into the request body. Splice point is before the first
`cache_control` marker if present (so the prefix stays cache-stable),
otherwise at the end of the messages array.

The `<kete:memory id="…">` ids are 8-char shortIDs derived from
`sha1(task.id)[:8]`. The MCP server (`kete mcp`) computes the same
ids, so the model can call `kete_expand id="…"` and resolve a memory
the proxy injected — even from a different process.

## Auto-compaction

A streaming usage tap watches Anthropic SSE `event: message_start`
and `event: message_delta` frames as they pass through, accumulating
token counts. Crossing the warn threshold
(`KETE_COMPACT_WARN_TOKENS`, default 160 000) kicks off a background
`compact.Compute` (Haiku via `compact_summary.txt`) that builds a
structured Summary. Crossing clear
(`KETE_COMPACT_CLEAR_TOKENS`, default 180 000) flips an
`applyPending` flag; the **next** request's body is rewritten by
`compact.Apply` to replace `messages` with
`[{role:user,content:<summary>}, {role:user,content:<next prompt>}]`.

This is the deliberate ADR 0006 exception: compaction re-marshals.
The cache prefix breaks by design — we are starting fresh.

## Expand-loop orchestrator

For non-streaming requests, the proxy runs a tool-loop. It buffers
the upstream response, parses for a `kete_expand` tool_use block,
resolves it via `store.FindByShortID(inject.ShortID, id)`, builds a
continue body (orig messages + assistant turn + tool_result), and
re-forwards. Cap: 5 cycles per request (brief 002 / plan 002 phase
16). On cap hit, the last response goes back to the client as-is.

Streaming requests skip the orchestrator — Crush dispatches
`kete_expand` client-side via the stdio MCP server. Both paths run
in production.

## Test surface

`go test ./internal/proxy/...` covers, end-to-end:

- `/health` (200), unknown route (404), body limit (413), timeout.
- Header sanitisation + redaction.
- Upstream selector precedence.
- Capture + injection + enrichment round-trip against an httptest
  fake upstream (`TestProxy_CaptureInjectAndEnrich`).
- Drift hook fires on Nth request, queues correction for (N+1)th,
  drift_log row written (`TestProxy_DriftFiresAndCorrectsNextRequest`).
- Compaction usage tap, threshold cross, body rewrite on next
  request (`TestProxy_CompactionFires_NextRequestRewritten`).
- Expand-loop resolution + 5-cycle cap +
  streaming pass-through (`TestExpandLoop_*`).

Live-verified on AWS Bedrock against
`us.anthropic.claude-haiku-4-5-20251001-v1:0`: non-streaming round
trip, streaming round trip, byte-exact memory injection (seeded
"kowhai" retrieved through the proxy).
