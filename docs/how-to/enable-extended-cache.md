# How-to: enable extended cache

Anthropic's prompt cache has a 5-minute TTL. When you pause to read
output, the next prompt often misses the cache and pays
`cache_creation` (1.25× base price, counts toward ITPM). The
extended-cache feature sends a minimal keep-alive every ~60s during
idle, refreshing the cache with the exact same prefix.

Per ADR 0013, this is **opt-in**. By using it you consent to kete
making API requests on your behalf.

## Enable

```sh
kete proxy --extended-cache
```

Or via env:

```sh
export KETE_EXTENDED_CACHE=true   # (planned)
kete proxy
```

(For 0.1.0 the flag is the only switch.)

## What kete does

After every successful forward to the **Anthropic upstream**, kete
stashes the raw request body + sanitised headers + upstream URL
keyed by project path. A background ticker fires every 60 s and:

1. Skips sessions idle for less than 4 minutes.
2. Drops sessions idle for more than 10 minutes total (cleanup).
3. Otherwise, byte-splices a single user message
   `,{"role":"user","content":"."}` before the closing `]` of the
   `messages` array — using `inject.AtMessages`, the exact same
   helper that handles memory injection. **Bytes before the splice
   point are unchanged**, which is what keeps the cache prefix
   matching.
4. Sends the keep-alive with the original headers (so auth still
   works) and discards the response.

Per-session cap: **2 keep-alives** before the session must see real
traffic again.

## What kete does not do

- **Bedrock keepalive.** Bedrock has its own cache and the cost
  curve is different; only the Anthropic upstream slot is wired.
  (`if up == UpstreamAnthropic` in `handleMessages`.)
- **cc-proxy keepalive.** cc-proxy is wire-identical to Anthropic
  but kete currently only stashes for `UpstreamAnthropic`. If a
  user wants this on cc-proxy, file a brief.
- **Streaming-aware retry.** The keep-alive is fire-and-forget; if
  Anthropic is down, the keep-alive fails silently and the cache
  decays. We accept that — the cache decaying is what would have
  happened anyway.

## Cost surface

Per the TS reference implementation: ~$0.002 per keep-alive on Haiku.
With the 2-per-period cap and a 60s ticker, worst case is ~$0.004
per 14-minute idle period (4 min before first fire, two fires within
the period, then the session evicts at 10 min total idle).

## Verify it's working

After 4+ minutes of idle, check:

```sh
ls -la ~/.kete/   # not the right surface for this; use the proxy stderr
```

Better: run with `--debug` (planned) and watch for `keepalive: fire`
log lines, or curl `/health` and inspect SQLite — `tasks` rows aren't
written for keep-alives, only for real client requests, so a stable
`tasks` count over 5+ minutes plus active cache_read on the next real
prompt is the signal.

## Cleanup

`SIGINT` or `SIGTERM` on `kete proxy` runs `keepalive.Manager.Close`
during graceful shutdown. No orphan keep-alives on crash either —
Anthropic just times out the cache, which is what would have
happened without us.
