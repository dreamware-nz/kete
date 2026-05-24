# How-to: use cc-proxy as the upstream

`dreamware-nz/cc-proxy` is a Swift menubar app that exposes an
Anthropic-compatible HTTP server backed by your Claude Code
subscription's OAuth credentials (Keychain). When kete is pointed at
cc-proxy, you get kete's capture/inject/drift/compaction layered on
top of cc-proxy's subscription-billing + reliability.

## What you need

- macOS (cc-proxy is macOS-only — Keychain).
- cc-proxy installed and running. By default it listens on
  `http://127.0.0.1:8787`.
- The shared-secret API key cc-proxy generated on first launch.
  In cc-proxy's UI: copy it from the menubar dropdown.

## Configure

```sh
export KETE_UPSTREAM=cc-proxy
export KETE_CC_PROXY_KEY="<the secret from cc-proxy>"
# optional, defaults to http://127.0.0.1:8787
export KETE_CC_PROXY_URL="http://127.0.0.1:8787"
./bin/kete proxy
```

Point your client at `http://127.0.0.1:8080` and use the model id
your subscription supports.

The client passes its `x-api-key: $KETE_CC_PROXY_KEY` header through
kete to cc-proxy; cc-proxy translates internally to a Bearer OAuth
token. **kete never sees the OAuth token.**

## What kete does

cc-proxy is wire-identical to Anthropic-direct from kete's
perspective: same body, same SSE, same prompt-cache semantics. The
adapter literally reuses `anthropic.Adapter` with a different base
URL — see `internal/adapter/ccproxy/`. ADR 0006's byte-exact rule
holds without exceptions.

## What kete does not do

- **No OAuth handling.** That's cc-proxy's job; kete forwards the
  shared-secret header.
- **No subscription accounting.** cc-proxy's menubar quota meter is
  the source of truth.
- **No fallback to direct API.** If cc-proxy is down, kete returns
  whatever cc-proxy's listener returns. Configure your client's
  fallback or restart cc-proxy.

## Limits

cc-proxy is macOS-only. Linux/Windows users would have to run
cc-proxy on a reachable Mac and point `KETE_CC_PROXY_URL` at it —
that's their problem to solve, not kete's.
