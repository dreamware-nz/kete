---
id: 013-cc-proxy-upstream
date: 2026-05-24
status: accepted
from-idea: 2026-05-24-cc-proxy-upstream
design: null
adrs: [0015-three-upstreams-selection]
plan: 013-cc-proxy-upstream
---

# 013 — cc-proxy as a third upstream

## Problem

`dreamware-nz/cc-proxy` is a Swift macOS menubar app that exposes an Anthropic-compatible HTTP server on `127.0.0.1:8787` (default) and forwards `/v1/messages` to `api.anthropic.com` using the Claude Code subscription's OAuth credentials from Keychain. It's the productionised sibling of `dreamware-nz/cc-api` (which was the working prototype). cc-proxy adds: OAuth Manager actor with single-flight refresh, billing-header injection, retry/backoff with decision-window semantics, optional OpenAI-compatible fallback after N overload retries, byte-for-byte SSE/JSON usage taps, persistent `usage.json`, and a menubar quota meter. See `dreamware-nz/cc-proxy/docs/PLAN.md` and ADRs 0001–0028 in that repo.

We want grov to know about cc-proxy as an upstream so a user on a Claude Code subscription can run `Crush → grov → cc-proxy → api.anthropic.com` and get grov's capture/inject/drift on top of cc-proxy's subscription-billing + reliability layer.

From grov's perspective, cc-proxy is wire-identical to Anthropic-direct: same body, same SSE, same prompt-cache semantics. The only differences are:

- Base URL is `http://127.0.0.1:8787` (configurable).
- Inbound auth is `x-api-key: <CCPROXY_API_KEY>` (cc-proxy's shared-secret, generated at cc-proxy's first launch and stored in macOS UserDefaults). cc-proxy translates internally to a Bearer OAuth token; kete never sees the OAuth token.
- The Anthropic prompt cache *does* work end-to-end. cc-proxy is byte-for-byte on the request path (per cc-proxy's own ADRs 0002 and 0007); kete's ADR 0006's discipline holds without exceptions.

cc-proxy is macOS-only by design (Keychain access). kete works fine on macOS too; if a Linux/Windows user wants `cc-proxy` upstream, they'd have to run cc-proxy on a reachable Mac. That's their problem, not ours.

## Boundary with cc-proxy

These two systems compose; they don't fight. The boundary is:

| Concern | grov | cc-proxy |
|---|---|---|
| Capture reasoning into team memory | ✓ | — |
| Inject prior reasoning into new sessions | ✓ | — |
| Drift detection / correction | ✓ | — |
| Auto-compaction | ✓ | — |
| OAuth refresh against `api.anthropic.com` | — | ✓ |
| Billing-header injection (subscription identity) | — | ✓ |
| Retry / backoff / decision window on overload | — | ✓ |
| OpenAI-compatible fallback when subscription is exhausted | — | ✓ |
| Token / quota tracking (5h / 7d windows) | — | ✓ |
| Menubar UI | — | ✓ |

grov captures *task reasoning* (what was decided and why); cc-proxy tracks *quota usage* (how many tokens, how close to limits). They observe the same traffic for different purposes. We do not duplicate cc-proxy's retry, taps, or usage accounting; we trust cc-proxy to handle the upstream's reliability.

A consequence of this boundary: if cc-proxy fails over to its OpenAI fallback (ADRs 0016–0021 in cc-proxy), grov sees that as a normal Anthropic-shaped response — cc-proxy synthesises Anthropic SSE framing from OpenAI chunks. Grov's adapter doesn't need to know.

## Who is hurt

- Users on a Claude Code subscription who want grov's capture/inject/drift features without paying twice. Today they pick: subscription via the `claude` CLI (no grov), or grov via metered API (ignores their subscription).
- Mixed teams where some devs are on subscription and some are on Bedrock. A single grov binary should serve both.
- Users running cc-proxy with fallback enabled who want grov's reasoning capture across both the subscription path and the fallback path. Without grov knowing about cc-proxy, the fallback's responses — once they reach grov — would still be capturable, but the user has no clean way to point Crush at "cc-proxy via grov" without a config override.

## Constraints

- **Wire compat with the Anthropic adapter.** No new adapter; cc-proxy is a base-URL + auth-header configuration of the existing Anthropic adapter.
- **cc-proxy is pluggable, not bundled.** We do not ship cc-proxy inside kete. cc-proxy has its own release, install, and lifecycle (it's a `.app` bundle the user starts from Finder or `make app && open`).
- **Default URL `http://127.0.0.1:8787`** matches cc-proxy's default. Override via `KETE_CC_PROXY_URL`.
- **Auth via `KETE_CC_PROXY_KEY`** env var (the value cc-proxy expects in `x-api-key`; in cc-proxy this is the auto-generated UUID surfaced in the menubar Settings window). Mirrors how cc-proxy is configured. We do not store this on disk; it's per-grov-instance.
- **No upstream cache assumption changes.** cc-proxy forwards byte-exact to Anthropic; the prompt cache works. Direct-API and cc-proxy are interchangeable from a cache perspective.
- **Do not duplicate cc-proxy features.** No retry layer, no usage tracking, no fallback decision in grov when targeting `cc-proxy`. If cc-proxy is doing its job, grov doesn't need to. If cc-proxy isn't running, the request fails; that's the right failure mode.

## Success looks like

- `kete proxy` with `KETE_UPSTREAM=cc-proxy` and `KETE_CC_PROXY_KEY=<key>` set: a Crush session through grov captures a task; the user sees the request in cc-proxy's request log; the response streams back identically to direct-API; the captured `task` row has the same shape as a direct-API session.
- A user with cc-proxy running on a non-default port can override via `KETE_CC_PROXY_URL=http://127.0.0.1:9999` without restarting grov mid-session (env-var-at-startup is fine).
- The same prompt sent twice hits Anthropic's prompt cache (verified via `cache_read_input_tokens` in usage). cc-proxy → grov adds no measurable overhead beyond two localhost hops.
- A grov instance configured for `cc-proxy` that loses connection to cc-api (cc-proxy crashed or wasn't started) returns a clear 502 with the underlying connect error, not a silent fallback to direct-API.
- When cc-proxy itself fails over to its OpenAI fallback (its ADRs 0016–0021), grov continues to capture; the captured `task.system_name` reflects the model the user requested (cc-proxy's translation is invisible to grov by design).

## Non-goals

- Bundling cc-proxy inside kete. Separate concerns, separate repos, separate lifecycles.
- Auto-starting cc-proxy from kete. If it isn't running, the user starts it (or sets up cc-proxy's "Launch at login", its ADR 0010).
- OAuth-token handling. cc-proxy owns that; we never touch the Keychain.
- Cross-platform OAuth. cc-proxy is macOS-only by intent; kete routes to whatever's reachable on the configured URL but doesn't promise cc-proxy works without macOS.
- Sharing usage state with cc-proxy's `usage.json`. cc-proxy reads `~/Library/Application Support/cc-proxy/usage.json`; grov's `~/.kete/memory.db` is unrelated. If a future feature wants to render quota in grov's `status` command, that's a new brief and a defined read-only contract on cc-proxy's file.
- Multiple cc-proxy instances behind kete (e.g. user + team subscriptions). One cc-proxy per kete for now.

## Open questions

- `[adr]` 0015 — Three-upstream selection rule (header > model-id > env): `anthropic-direct | cc-proxy | bedrock`. Per-request override header, model-id pattern, env var. Accepted.
- Whether the model-id pattern can distinguish cc-proxy from anthropic-direct. It can't — same body shape, same model ids. So cc-proxy must be selected by header or env, never by auto-detection. (Captured in 0018.)
- Whether `kete doctor` should sanity-check cc-proxy reachability and surface its quota numbers (read-only) in the diagnostic output. Pro: closes the loop on the most common failure ("cc-proxy not running"). Con: introduces a read-only dependency on cc-proxy's `usage.json` schema. Defer to plan stage; ship reachability check first, quota mirror later.
- How `kete status` (or a new subcommand) might surface "this task was served by cc-proxy with OpenAI fallback" if cc-proxy decides to expose that on its taps. Pure read; pure UX. Defer.

## Doc impact

- `docs/how-to/use-cc-proxy.md` `[new]` — install cc-proxy, copy its API key, set `KETE_UPSTREAM=cc-proxy` + `KETE_CC_PROXY_KEY`, run kete.
- `docs/reference/env.md` `[update]` — `KETE_UPSTREAM=cc-proxy`, `KETE_CC_PROXY_URL`, `KETE_CC_PROXY_KEY`.
- `docs/explanation/three-upstreams.md` `[new]` (replaces the planned `two-upstreams.md`) — comparison table including cc-proxy column (cache: yes; auth: subscription via cc-proxy OAuth; latency: localhost double-hop; reliability: handled by cc-proxy's retry/fallback).
- `docs/explanation/grov-and-cc-proxy.md` `[new]` — the responsibility boundary in this brief, expanded with examples.
- `README.md` `[update]` — quickstart paragraph noting subscription-via-cc-proxy as a supported configuration; link to `dreamware-nz/cc-proxy`.
