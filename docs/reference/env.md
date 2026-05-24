# Environment reference

> All kete configuration is via environment variables. No config file.

## Core paths

| Var               | Default       | Effect                                                   |
| ----------------- | ------------- | -------------------------------------------------------- |
| `KETE_HOME`       | `~/.kete`     | Override the dotdir (logs, DB if `KETE_DB_PATH` unset)   |
| `KETE_DB_PATH`    | `~/.kete/memory.db` | Full DB path override                              |

## Proxy

| Var                          | Default     | Effect                              |
| ---------------------------- | ----------- | ----------------------------------- |
| `KETE_HOST`                  | `127.0.0.1` | Proxy bind host                     |
| `KETE_PORT`                  | `8080`      | Proxy bind port                     |
| `KETE_PROJECT`               | (none)      | Project key for capture + inject    |
| `KETE_INJECT_MEMORY`         | `false`     | Splice prior memories into requests |
| `KETE_DRIFT_ENABLED`         | `false`     | Run drift scoring + correction      |
| `KETE_DRIFT_CHECK_INTERVAL`  | `5`         | Fire drift check every Nth prompt   |
| `KETE_COMPACT_WARN_TOKENS`   | `160000`    | PreCompute fires at this usage      |
| `KETE_COMPACT_CLEAR_TOKENS`  | `180000`    | Apply fires at this usage           |
| `KETE_HARD_TRUNCATE_BYTES`   | `1048576`   | Drop middle of `messages` at/above this body size |
| `KETE_HARD_TRUNCATE_KEEP`    | `30`        | Number of recent messages to retain on hard-truncate |
| `KETE_CAPTURE_MIN_BYTES`     | `2048`      | Skip capture when raw body is smaller than this (filters Crush keepalive pings, title-gen, autocomplete) |
| `KETE_EXTENDED_CACHE`        | `false`     | Opt into the keep-alive (ADR 0013)  |

### Project keying

The proxy needs a project key to do anything useful (capture, memory
injection, drift). Resolution per request:

1. `X-Kete-Project` HTTP header on the inbound request (per-request).
2. `KETE_PROJECT` env var on the daemon (per-daemon).
3. Empty — the daemon is a passthrough proxy with the hard-truncate
   safety net only. Capture, injection, and drift all skip.

There is **no fallback to the daemon's cwd**: under launchd, that
would silently bucket every project on the machine into one identity.
Either run one daemon per project (unique `KETE_PORT` + `KETE_PROJECT`
per plist) or have your client send `X-Kete-Project`.

### Memory + drift kill switches

`KETE_INJECT_MEMORY` and `KETE_DRIFT_ENABLED` default to off while
project resolution and extraction quality stabilise. Flip to `1`
on a per-daemon basis when you trust the captured rows for that
project.

## Upstream selection (ADR 0015)

| Var                  | Default                       | Effect                                  |
| -------------------- | ----------------------------- | --------------------------------------- |
| `KETE_UPSTREAM`      | `anthropic`                   | `anthropic` \| `cc-proxy` \| `bedrock`  |
| `KETE_ANTHROPIC_URL` | `https://api.anthropic.com`   | Override the Anthropic-direct base URL  |
| `KETE_CC_PROXY_URL`  | `http://127.0.0.1:8787`       | cc-proxy base URL                       |
| `KETE_CC_PROXY_KEY`  | (none)                        | cc-proxy shared secret; required        |
| `AWS_REGION`         | (none)                        | Bedrock region; required for that route |

AWS credentials resolve via the standard SDK chain (env vars,
`~/.aws/credentials`, SSO, IRSA, instance profile). kete never reads
them directly.

## Extraction (Haiku)

| Var                 | Default                        | Effect                            |
| ------------------- | ------------------------------ | --------------------------------- |
| `ANTHROPIC_API_KEY` | (none)                         | Required for extraction (always Anthropic-direct, even when proxy upstream is Bedrock or cc-proxy) |
| `KETE_DRIFT_MODEL`  | `claude-haiku-4-5-20251001`    | Override the extraction model    |
