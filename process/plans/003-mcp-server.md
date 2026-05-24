---
id: 003-mcp-server
date: 2026-05-24
status: done
brief: 003-mcp-server
design: null
adrs: [0012-mcp-stdio-jsonrpc-library, 0008-mcp-tools-and-descriptions]
---

# 003 — MCP server

## Goal

A stdio MCP server (`kete mcp`) that speaks JSON-RPC 2.0 and exposes `kete_preview` and `kete_expand` against the local store.

## Phases

### Phase 1 — JSON-RPC 2.0 framer

- **Outcome:** `Content-Length:`-framed reads from stdin; framed writes to stdout; logs to stderr only.
- **Slice:** `internal/mcp/framing.go`.
- **Context:** ADR 0012.
- **Depends-on:** `[]`
- **Done when:** unit test round-trips a `ping` request.

### Phase 2 — `initialize` handshake

- **Outcome:** Responds to MCP `initialize` with name/version + tool capability.
- **Slice:** `internal/mcp/handshake.go`.
- **Context:** `internal/mcp/framing.go`; MCP spec.
- **Done when:** test client gets a valid `initializeResult`.

### Phase 3 — `tools/list` returns the two tools

- **Outcome:** Returns `kete_preview` and `kete_expand` with embedded descriptions per ADR 0008.
- **Slice:** `internal/mcp/tools.go`; descriptions in `prompts/mcp_*.txt` via `go:embed`.
- **Context:** ADR 0008; `internal/mcp/handshake.go`.
- **Done when:** `tools/list` returns both with JSON Schema.

### Phase 4 — `kete_preview` impl

- **Outcome:** Given `query` + `project_path`, returns top-N previews.
- **Slice:** `internal/mcp/preview.go`; uses `store.ListTasks`/`SearchTasks`.
- **Context:** `internal/mcp/tools.go`; `internal/store/tasks.go`.
- **Depends-on:** `[phase-3]`
- **Done when:** preview against seeded DB returns expected ids.

### Phase 5 — Per-process preview cache

- **Outcome:** 8-char id maps to a stable preview set; lifetime: process.
- **Slice:** `internal/mcp/cache.go`.
- **Context:** `internal/mcp/preview.go`.
- **Depends-on:** `[phase-4]`
- **Done when:** preview returns id; cache survives across calls.

### Phase 6 — `kete_expand` impl

- **Outcome:** Given an id, returns full task body.
- **Slice:** `internal/mcp/expand.go`.
- **Context:** `internal/mcp/cache.go`; `internal/store/tasks.go`.
- **Depends-on:** `[phase-5]`
- **Done when:** expand of preview-id returns full task.

### Phase 7 — Wire `kete mcp` to the framer

- **Outcome:** Replace plan 001 phase 5's stub: open store, run loop until EOF, close.
- **Slice:** rewrite `cmd/kete/mcp.go`.
- **Context:** `cmd/kete/store.go`; `internal/mcp/*`.
- **Depends-on:** `[phase-1, phase-2, phase-3, phase-4, phase-6]`
- **Done when:** end-to-end manual test with a real MCP client lists and calls both tools.

### Phase 8 — Doc: `docs/reference/mcp.md`

- **Outcome:** Tool surface, JSON Schema, example call/response.
- **Slice:** new file.
- **Context:** `internal/mcp/tools.go`; brief 003 Doc impact.
- **Depends-on:** `[phase-3]`
- **Done when:** file exists; cited from `docs/explanation/why-proxy-not-just-mcp.md`.

## Out of scope

- Tools beyond preview/expand. SSE/HTTP transport. Non-Claude tool descriptions.

## Assumptions

- Hand-rolled JSON-RPC stays short (ADR 0012). Plan 004's store API is sufficient. The 5-cycle expand cap lives in the proxy, not here.
