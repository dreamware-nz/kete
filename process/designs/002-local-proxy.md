---
id: 002-local-proxy
date: 2026-05-24
status: accepted
brief: 002-local-proxy
adrs: [0005-http-server-net-http-chi, 0006-raw-body-passthrough-for-prompt-cache, 0007-agent-agnostic-adapter-interface, 0009-haiku-as-extraction-model, 0011-drift-correction-four-levels, 0013-byte-exact-keepalive-injection]
plan: null
---

# Local proxy — design

## Principles

- **Be transparent.** Anything the user could observe by pointing their tool directly at the upstream API must look the same when pointed at us, except for the things this proxy exists to do.
- **Bytes are sacred in the request path.** A re-serialised JSON is a cache miss. A cache miss is real money.
- **Failure of an enrichment is non-fatal.** Drift extraction, memory injection, summary generation — every one of them can be skipped without breaking the request. They never block forwarding.
- **One state machine per session, not one mega-orchestrator.** The TS code's 1218-line `orchestrator.ts` is a smell we don't have to inherit.
- **Two model vendors, one shape.** Anthropic and Codex differ in wire format and a few semantic ops. Everything else is shared.

## Considerations

Prior art the design draws on:

- The TS implementation, especially `Grov-Original/src/integrations/proxy/` (server, orchestrator, agents, handlers, cache, injection).
- `Grov-Original/docs/plan_proxy_local.md` — the original design plan; its database section is largely accurate, its "FILE SUMMARY" reflects an earlier file layout.
- `Grov-Original/docs/extended_cache.md` — the byte-exact discipline lives here.
- Anthropic's prompt-cache documentation (cache_control breakpoints, `cache_creation` vs `cache_read` pricing, ITPM accounting).
- Go HTTP idioms: `net/http`, `chi`, context propagation, graceful shutdown via `http.Server.Shutdown`.

Constraints carried in from briefs and ADRs:

- Bind `127.0.0.1:8080` defaults; env override; one binary, no TLS at the proxy.
- Body limit 10 MB; request timeout 5 minutes; same as TS.
- Forward header whitelist `[x-api-key, authorization, anthropic-version, content-type, anthropic-beta]`; never log `x-api-key` or `authorization` in plaintext.
- Drift checks every `DRIFT_CHECK_INTERVAL=5` prompts; auto-compact warn / clear thresholds 160k / 180k tokens.
- Hard cap of 5 cycles on the `kete_expand` tool loop.
- Graceful shutdown: track active sockets, force-close after 500 ms.

## Integrated picture

### System sketch

```
┌────────────────────────────────────────────────────────────────────┐
│  Client (Claude Code / Codex CLI)                                  │
│        │                                                           │
│        ▼  POST /v1/messages | /v1/responses                        │
│  ┌──────────────────────────────────────────────────────────┐      │
│  │  Server (net/http + chi)                                 │      │
│  │  ─ rawBody middleware (read into []byte, attach to ctx)  │      │
│  │  ─ /health, /v1/messages, /v1/responses                  │      │
│  │  ─ 404 catch-all                                         │      │
│  └──────────────────────────────────────────────────────────┘      │
│        │                                                           │
│        ▼   ctx{ rawBody, vendor }                                  │
│  ┌──────────────────────────────────────────────────────────┐      │
│  │  Router → Adapter (Wire + Semantics)                     │      │
│  │  ─ vendor selected by URL path                           │      │
│  └──────────────────────────────────────────────────────────┘      │
│        │                                                           │
│        ▼                                                           │
│  ┌──────────────────────────────────────────────────────────┐      │
│  │  Pipeline (per request)                                  │      │
│  │   1. preprocess    — memory injection, intent extract    │      │
│  │   2. forward       — adapter.Wire.Forward                │      │
│  │   3. tool-loop     — kete_expand cycles (max 5)          │      │
│  │   4. postprocess   — drift score, step write, capture    │      │
│  │   5. extended-cache— stash rawBody+headers if enabled    │      │
│  └──────────────────────────────────────────────────────────┘      │
│        │                                                           │
│        ▼                                                           │
│  ┌──────────────────────────────────────────────────────────┐      │
│  │  Stores                                                  │      │
│  │   ─ SQLite (sessions, steps, tasks, drift_log, …)        │      │
│  │   ─ in-memory: extendedCache, sessionRouting, lastDrift  │      │
│  └──────────────────────────────────────────────────────────┘      │
│        │                                                           │
│        ▼                                                           │
│  Upstream (Anthropic | OpenAI)                                     │
└────────────────────────────────────────────────────────────────────┘
```

### Surface

Three HTTP routes, exactly:

| Method | Path             | Vendor    | Purpose                             |
| ------ | ---------------- | --------- | ----------------------------------- |
| GET    | `/health`        | —         | Liveness; returns `{"status":"ok"}` |
| POST   | `/v1/messages`   | Anthropic | Forwarded to `api.anthropic.com`    |
| POST   | `/v1/responses`  | OpenAI    | Forwarded to OpenAI                 |
| *      | `/*`             | —         | 404 with `{"error":"Not found"}`    |

Configuration is exclusively via environment variables (see `docs/reference/env.md`); there is no config file. Defaults match TS.

### Data shapes

The internal data shapes the proxy traffics in:

```go
type RawBody []byte                  // exact bytes; never re-marshaled

type RequestView struct {            // typed read-only view; cheap to recreate
    Model       string
    Messages    []Message            // []Message is vendor-portable
    Tools       []ToolDef
    MaxTokens   int
    Stream      bool
    SystemPrompt SystemPrompt        // string or list of blocks (Anthropic)
    CacheControl []CacheControlMarker // positions of cache_control in the body
}

type ResponseEvent interface { ... } // streaming union: TextDelta, ToolUse, EndTurn, Usage

type SessionState struct {           // mirrors session_states table
    ID                string
    ProjectPath       string
    Goal              string
    ExpectedScope     []string
    Constraints       []string
    Mode              SessionMode    // normal | drifted | forced
    EscalationCount   int
    LastDriftScore    int
    PendingCorrection string
    TokenCount        int
    LastClearAt       *time.Time
}

type Step struct {                   // mirrors steps table
    ID, SessionID    string
    ActionType       ActionType
    Files, Folders   []string
    Command          string
    Reasoning        string
    DriftScore       int
    DriftType        DriftType
    CorrectionLevel  CorrectionLevel
    Timestamp        time.Time
}
```

### Behaviour over time — single request lifecycle

```
client          server          adapter         upstream        stores
  │  POST           │              │              │              │
  │ ───────────────►│              │              │              │
  │                 │ rawBody      │              │              │
  │                 │ middleware   │              │              │
  │                 │              │              │              │
  │                 │ pipeline.Run(ctx, rawBody, adapter)        │
  │                 │              │              │              │
  │                 │  ── preprocess ──            │              │
  │                 │   load session ─────────────────────────────►│
  │                 │   inject memory              │              │
  │                 │   (mutate rawBody in-place)  │              │
  │                 │              │              │              │
  │                 │  ── forward ──               │              │
  │                 │              │ Forward(rawBody)─────────────►│
  │                 │              │ ◄──── stream of events       │
  │                 │  ── tool-loop ──             │              │
  │                 │   if kete_expand requested:  │              │
  │                 │     fetch memory ────────────────────────────►│
  │                 │     buildContinue(rawBody, …)│              │
  │                 │     loop (max 5)             │              │
  │                 │              │              │              │
  │                 │  ── postprocess ──           │              │
  │                 │   parse actions              │              │
  │                 │   drift score (Haiku) ──────────────────────►│
  │                 │   write Step ────────────────────────────────►│
  │                 │   on end_turn:               │              │
  │                 │     summarize ──────────────────────────────►│
  │                 │     write Task ──────────────────────────────►│
  │                 │   if extended-cache enabled: │              │
  │                 │     stash rawBody+headers    │              │
  │ ◄────────────── │   stream response back       │              │
```

### Pipeline as small functions

The pipeline is a struct with named phases, not a 1218-line method. Each phase has a typed signature and is independently testable.

```go
type Pipeline struct {
    Store      store.DB
    Cloud      cloud.Client
    Extractor  extraction.Haiku
    KeepAlive  *extendedcache.Cache
    Adapters   map[Vendor]adapter.Adapter
}

func (p *Pipeline) Handle(w http.ResponseWriter, r *http.Request) {
    raw := raw_body.From(r.Context())
    a   := p.Adapters[vendor.From(r.URL.Path)]

    ctx := newPipelineCtx(r.Context(), raw, a)

    if err := p.preprocess(ctx);  err != nil { p.fail(w, err); return }
    if err := p.forward(ctx, w);  err != nil { p.fail(w, err); return }   // streams to client
    if err := p.toolLoop(ctx, w); err != nil { p.fail(w, err); return }   // up to 5 cycles
    p.postprocess(ctx)                                                    // best-effort
}
```

`postprocess` runs synchronously after the response stream completes (so we can write `Step` rows reflecting actions that happened in this turn) but its sub-steps (drift, summary, capture) are independent: a failure in one doesn't abort the others.

### kete_expand tool loop

When the model returns a `tool_use` for `kete_expand`, the pipeline:

1. Pulls the requested 8-char ID from the in-memory preview cache (or the cloud if not cached).
2. Calls `adapter.Semantics.BuildContinueBody(req, resp, toolID, toolResult)` to build a follow-up request.
3. Re-enters `forward` with the new body.
4. Increments a counter; aborts the loop and returns the model's last response if the counter exceeds 5.

The 5-cycle cap is a hard constant in code, not a config knob. It exists because uncapped, a pathological model can spend an entire context window cycling through memories.

### Extended cache lifecycle

Enabled only when `--extended-cache` / `KETE_EXTENDED_CACHE=true`.

A goroutine started at server boot ticks every 60 s. For each entry in the in-memory `extendedCache.Map[sessionID]Entry`:

- If `now - timestamp < 4min`: skip (still active).
- If `now - timestamp >= 10min`: drop (cleanup).
- Else if `keepAliveCount < 2`: emit a keep-alive (per ADR 0013), increment counter, update timestamp.

On `SIGINT`/`SIGTERM`, the goroutine receives a stop signal, the cache is cleared, and the server runs `Shutdown(ctx)` with a 500 ms force-close.

### Drift detection integration

- Drift checks are skipped if the current turn's action count is zero (model produced no actions; nothing to score).
- A drift result with `score >= 8` writes a normal `Step` and updates `last_drift_score`.
- A drift result with `score < 5` writes a `drift_log` entry instead of a `Step` (the action is "rejected"), increments `escalation_count`, and queues a correction for injection on the next request.
- At `escalation_count >= 3`, the next correction is a Haiku-built "forced recovery" prompt that names a specific first action.

The drift score is the same Haiku call used by extraction (ADR 0009); the prompts live in `internal/extraction/prompts/`.

### Auto-compaction integration

- `TokenCount` is updated from each response's usage block.
- When `TokenCount >= 160_000` (`TOKEN_WARNING_THRESHOLD`), schedule a Haiku call to pre-compute a summary.
- When `TokenCount >= 180_000` (`TOKEN_CLEAR_THRESHOLD`), inject the summary as a fresh user message, clear the conversation, and reset the cache prefix.

The summary call shares the `extraction.Haiku` client; its prompt is in `internal/extraction/prompts/compaction_summary.txt`.

## Trade-offs

- **Pipeline as named phases vs one big function.** We prefer the named phases. Cost: more interfaces and slightly more indirection. Benefit: each phase is independently testable; the orchestrator is no longer the place where every bug lives. The TS 1218-line file is the cautionary tale.
- **Two-interface adapter (Wire + Semantics) vs one fat interface.** Discussed and decided in ADR 0007. Cost is two source files per vendor. Benefit is honest separation between bytes and meaning.
- **Synchronous postprocess vs async.** Synchronous keeps the captured `Step` causal with the response; async would let the response return faster but loses the guarantee that the next request sees this turn's `Step`. We accept the slight extra latency.
- **In-memory routing state vs DB-backed.** `activeSessions`, `lastDriftResults`, `lastMessageCount`, `extendedCache` are all in memory. A proxy restart loses them. That's tolerable: sessions reconnect; drift state lost just means the next correction starts at level 1; extended-cache is opt-in. We do not introduce DB state we'd need to expire.
- **Logging via `slog` to a file vs `pino`-shaped JSON.** `slog` to `~/.kete/kete-proxy.log` is enough; the dashboard does not consume proxy logs.

## Open design questions

- Whether to expose a `/v1/sessions` debug surface (read-only, listing active sessions) for `kete proxy-status`. The TS implementation uses a separate command that opens the DB directly. Cleaner long-term to have an HTTP surface; not a v1 concern.
- Whether to add a strict-mode flag that fails the request if memory injection fails (rather than skipping). The current TS behaviour is "skip on failure"; we keep that. A strict mode might suit CI environments later.
- How to expose extended-cache metrics (hit/miss, keep-alives sent, $ saved) without adding telemetry. Probably a JSON file under `~/.kete/extended-cache.stats.json` written on shutdown.

## Revisions

(append-only after acceptance)
