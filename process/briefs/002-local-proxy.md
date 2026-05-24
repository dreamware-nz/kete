---
id: 002-local-proxy
date: 2026-05-24
status: accepted
from-idea: 2026-05-24-kete
design: 002-local-proxy
adrs: [0005-http-server-net-http-chi, 0006-raw-body-passthrough-for-prompt-cache, 0007-agent-agnostic-adapter-interface, 0009-haiku-as-extraction-model, 0011-drift-correction-four-levels, 0013-byte-exact-keepalive-injection, 0015-three-upstreams-selection]
plan: 002-local-proxy
---

# 002 — Local proxy

## Problem

kete needs to capture reasoning from a Crush session, inject prior reasoning into a new one, detect drift while a session runs, and auto-compact when the context window fills. To do any of that *reliably*, kete has to be in the network path between Crush and the upstream model API. A local HTTP proxy on `127.0.0.1:8080` that intercepts `POST /v1/messages` (Anthropic-shaped — direct, cc-proxy, or Bedrock) is that path.

This brief is the heart of kete. It is also the most byte-pedantic part: Anthropic's prompt cache matches requests by a prefix ETag, so any code that re-serialises a parsed JSON body invalidates the cache for the rest of the conversation. ADR 0006 captures the discipline; this brief implements it.

## Why a proxy and not just MCP

The most reasonable-sounding alternative is "skip the proxy; use Crush hooks for capture and an MCP server for retrieval". That alternative was considered explicitly in the first-principles review. It loses for one reason: **everything else in Crush's surface is cooperative.**

| Surface | Who decides whether it runs | Failure mode |
|---|---|---|
| Crush hooks | Crush, on lifecycle events Crush exposes | Crush adds new event we can't see; or removes one we depended on |
| MCP tool call | The model, when the model decides | Small / cheap / distracted models routinely ignore "MANDATORY" tool descriptions |
| HTTP request | Nobody. The bytes have to flow somewhere. | None we can be locked out of |

Hooks are a contract between us and Crush. MCP tools are a contract between us and the model. Both can fail silently. The proxy is a contract between us and **the wire** — the request *has* to traverse it, regardless of what Crush decided to surface or what the model decided to call.

Concretely:

- **Capture cannot rely on the model calling a tool.** Crush sessions on weak models would never capture.
- **Injection benefits from being unconditional.** The proxy injects prior reasoning into the request body before the model sees the prompt; the model can't "forget" to fetch context.
- **Drift correction has to be inline.** Once the agent has committed an action, our only remaining lever is to inject a correction *into the next request*, byte-mutating before forwarding. No hook gets us there.
- **Auto-compaction has to rewrite the request.** The conversation history *is* the request body. Compaction is a body rewrite, full stop.

The MCP server (brief 003) still ships, as belt-and-braces: when the model is smart enough to call `kete_expand` reliably, it complements the proxy by letting the model fetch full reasoning on demand. Neither layer depends on the other; both run.

## Who is hurt

- **dreamware-nz developers using Crush** — without the proxy, capture and drift are unreliable, and the user notices nothing when they fail.
- **The model's behaviour budget** — uncaught drift wastes tokens and time; injected memory cuts re-exploration. The proxy is the only place we can do either reliably.
- **Future contributors** working in the proxy's body-injection path. The byte-exact rule is easy to violate accidentally; the brief and ADR 0006 exist so a code review has something to point at.

## Constraints

- Bind defaults: `127.0.0.1:8080`. Override via `KETE_HOST` / `KETE_PORT`.
- Endpoints: `POST /v1/messages` (all three upstreams; Anthropic-shaped) plus `GET /health`. Everything else 404. (We do not implement OpenAI's `/v1/responses` for v1 — Crush doesn't need it.)
- Body limit `10 MB`, request timeout `5 min`.
- Forwarded request headers whitelist: `[x-api-key, authorization, anthropic-version, content-type, anthropic-beta]`. `x-kete-upstream` (routing override) is consumed and stripped, never forwarded.
- Secrets never logged: `x-api-key`, `authorization`, anything matching the AWS-credential header pattern.
- Anthropic prompt-cache prefix preservation: when injecting into a body that includes a `cache_control` block, bytes before the breakpoint must be byte-identical to what Crush sent. (ADR 0006.)
- `kete_expand` tool loop hard cap: **5 cycles** per request. Past that, return the model's last response and let it continue.
- Drift detection runs every `KETE_DRIFT_CHECK_INTERVAL` prompts (default `5`).
- Auto-compaction warning / clear thresholds at `160_000` / `180_000` tokens (defaults; overridable via env).
- Graceful shutdown: track active sockets, force-close after 500 ms.
- Three-upstream routing per ADR 0015: header `x-kete-upstream` → model-id pattern → `KETE_UPSTREAM` env var.

## Success looks like

- A Crush session pointed at `http://127.0.0.1:8080` completes a multi-prompt task. The captured `task` row in `~/.kete/memory.db` includes the goal, key decisions, files touched, and reasoning trace.
- A second Crush session against the same project receives the prior task injected into its first request. The model's first response references the prior decision rather than re-investigating.
- `kete drift-test "<prompt>" --goal "<goal>"` produces a coherent score and correction message on a fixture session.
- A request that injects two memories has a stable byte prefix across consecutive prompts in the same conversation (verifiable via `cache_read_input_tokens` in usage).
- A request with `x-kete-upstream: bedrock` forwards via SigV4 to Bedrock; one with `x-kete-upstream: cc-proxy` forwards to `127.0.0.1:8787`; default behaviour forwards to `api.anthropic.com`.
- `wrk -d30s` against `GET /health` doesn't observably affect a concurrent Crush session.

## Non-goals

- **Multi-client design.** Crush is the user. We don't ship Cursor/Zed/Codex CLI compatibility speculatively.
- **OpenAI `/v1/responses` endpoint.** Not needed for Crush + the three Anthropic-shaped upstreams.
- **TLS at the proxy.** `127.0.0.1` only.
- **Rate limiting at the proxy.** cc-proxy already does rate-aware backoff; Bedrock has its own; direct API has its own. Three upstream-side mechanisms is enough.
- **Telemetry.** No outbound calls beyond the upstream forward and the Haiku extraction calls.
- **Mid-session model-swap accounting.** dreamware-nz workflow doesn't swap; cc-proxy hides swaps. We capture against the model the session ends on.

## Open questions

All settled in existing ADRs:

- ADR 0005 — `net/http` + `chi`.
- ADR 0006 — raw-body passthrough discipline.
- ADR 0007 — split adapter (`Wire` + `Semantics`).
- ADR 0011 — four drift correction levels.
- ADR 0013 — byte-exact keep-alive injection.
- ADR 0014 — Bedrock SigV4 + event-stream demux.
- ADR 0015 — three-upstream selection.

The design doc `designs/002-local-proxy.md` covers the orchestrator state machine, the `kete_expand` tool loop, drift integration, extended-cache lifecycle, and graceful shutdown.

## Doc impact

- `docs/explanation/why-proxy-not-just-mcp.md` `[new]` — the cooperative-vs-non-cooperative argument above.
- `docs/explanation/raw-body-preservation.md` `[new]`.
- `docs/explanation/three-upstreams.md` `[new]`.
- `docs/reference/proxy.md` `[new]`.
- `docs/how-to/enable-extended-cache.md` `[new]`.
- `docs/how-to/use-cc-proxy.md` `[new]`.
- `docs/how-to/use-bedrock.md` `[new]`.
- `README.md` `[update]` — proxy quickstart.
