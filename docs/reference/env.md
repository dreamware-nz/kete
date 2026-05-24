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
| `KETE_PROJECT`               | cwd         | Project key for capture + inject    |
| `KETE_DRIFT_CHECK_INTERVAL`  | `5`         | Fire drift check every Nth prompt   |
| `KETE_COMPACT_WARN_TOKENS`   | `160000`    | PreCompute fires at this usage      |
| `KETE_COMPACT_CLEAR_TOKENS`  | `180000`    | Apply fires at this usage           |
| `KETE_EXTENDED_CACHE`        | `false`     | Opt into the keep-alive (ADR 0013)  |

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
