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

Extraction (plan 011) fills in `goal`, `decisions`, and
`files_touched` later, asynchronously.

## Test surface

`go test ./internal/proxy/...` covers, end-to-end:

- `/health` (200), unknown route (404), body limit (413), timeout.
- Header sanitisation + redaction.
- Upstream selector precedence.
- Capture + injection round-trip against an httptest fake upstream
  (asserts the upstream sees the injected body and a `proxy` row
  lands in the DB).
- Drift / compact hook firing patterns and 5-cycle expand guard.
