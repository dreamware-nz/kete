---
number: 0005
title: HTTP server uses net/http with go-chi for the proxy
date: 2026-05-24
status: accepted
brief: 002-local-proxy
supersedes: null
superseded-by: null
---

# 0005 — HTTP server uses net/http with go-chi for the proxy

## Context

The proxy needs to bind on `127.0.0.1:8080`, route three paths (`POST /v1/messages`, `POST /v1/responses`, `GET /health`, plus a 404 catch-all), preserve raw request bodies (ADR 0006), forward to upstream APIs, and stream SSE responses back. The TS code uses Fastify with a custom `addContentTypeParser` that parses to a buffer.

Three honest options in Go:

- `net/http` plus a small router (`go-chi/chi/v5`).
- `fasthttp` plus its router.
- `net/http` alone, with a hand-rolled `switch r.URL.Path`.

The proxy holds the raw request body alongside a parsed view of it (for inspection during injection / drift / extended-cache). `fasthttp`'s zero-copy `RequestCtx` is excellent for general perf but its API is built around reusing buffers across requests, which conflicts with "hold the bytes for the duration of the upstream round-trip". Trying to make `fasthttp` *not* reuse buffers means you fight its design. `net/http` makes raw-body capture trivial: read once into a `[]byte`, set it on a context value, hand a `bytes.Reader` to anything that wants to parse.

Between bare `net/http` and `chi`, the code shape is small enough that bare `net/http` is honest. But `chi` brings tiny things — middleware, param routing, a clean way to mount the catch-all — that are worth ~200 LOC of bespoke code. Sandi Metz on "small dependencies that earn their place": chi earns its place; gin/echo do not.

## Decision

The proxy uses `net/http` for the core server and `github.com/go-chi/chi/v5` for routing. Custom middleware reads the request body once into a `[]byte`, stashes both that and a parsed view on `request.Context()` via a typed key, and passes control to the route handler.

We do not use `fasthttp`. We do not use `gin`, `echo`, `fiber`.

## Options considered

- **`net/http` + `chi`.** What we picked. Idiomatic; the raw-body pattern is straightforward; tooling and middleware ecosystems are mature.
- **`net/http` alone.** Honest; saves a dependency. We'd reimplement chi's mount/`With`/middleware pattern badly. Reject.
- **`fasthttp`.** Faster on hello-world benchmarks. Fights raw-body preservation. Reject.
- **`gin` / `echo` / `fiber`.** Bigger surface; less idiomatic; we don't need their features. Reject.

## Consequences

Easier:

- Body capture: one middleware, one `httputil.DumpRequest`-style read into `[]byte`. No fight with the framework.
- SSE responses: `http.ResponseWriter` + `http.Flusher` is straightforward.
- Graceful shutdown: `http.Server.Shutdown(ctx)` with a `time.AfterFunc` for the 500 ms force-close (matches TS).
- Test ergonomics: `httptest.NewRequest` / `httptest.NewRecorder` work without ceremony.

Harder:

- We pay one more dependency (`chi`). Trivially worth it.
- `chi` doesn't give us streaming hooks the way Fastify does for content-type parsers; that's fine because we want to parse in middleware anyway.

If a real perf wall ever shows up under profiler — measured, not imagined — moving to `fasthttp` would mean re-doing the body-capture middleware. We will not preempt.
