---
number: 0012
title: MCP server is hand-rolled JSON-RPC 2.0 over stdio; capture is poll-based, not hook-based
date: 2026-05-24
status: accepted
brief: 003-mcp-server
supersedes: null
superseded-by: null
---

# 0012 — MCP server is hand-rolled JSON-RPC 2.0 over stdio; capture is poll-based, not hook-based

## Context

Two related questions for the MCP subsystem:

1. **Library or hand-rolled?** The TS code uses `@modelcontextprotocol/sdk`. A Go SDK exists at `github.com/modelcontextprotocol/go-sdk` but is at the time of this port still pre-1.0 and shifting (initialise/shutdown semantics, transport abstractions, error shapes). The protocol surface we use is small: stdio transport, tool list, tool call. JSON-RPC 2.0 over a length-prefixed stdio framing is ~150 lines of honest Go.

2. **Capture mechanism.** The TS code originally used Claude Code hooks (`SessionStart`, `Stop`, `UserPromptSubmit` writing to `additionalContext`). Those were removed (`docs/SESSION_DEC2_2025_HOOKS_REMOVAL.md`) in favour of polling-based capture (`cli-watcher.ts`, `cli-extractor.ts`) plus IDE-specific scanners (Antigravity). Hooks had a fundamental timing problem: by the time a hook fires, the model has already taken several actions, so corrections lag. Polling has the same lag for reads but doesn't pretend to do real-time correction — that job belongs to the proxy.

## Decision

The Go MCP server is hand-rolled. Wire is: read JSON-RPC messages from stdin (one per `\n`-delimited line, per the MCP spec's stdio transport), write replies to stdout. Logging goes to `~/.kete/kete-mcp.log`; nothing is ever written to stdout/stderr from the request path. The implementation lives in `internal/mcp/`; the JSON-RPC plumbing in `internal/mcp/jsonrpc.go` is ~200 lines.

Capture is poll-based. We do not implement Claude Code hooks. The capture sources are:

- IDE-mode capture (Cursor, Zed) — tail the IDE's chat log directory (`~/.cursor/chats`, `~/.zed/agents/...`) on a 5-second timer.
- CLI-mode capture (Cursor CLI, Codex CLI standalone) — same poller, different paths.
- Antigravity — the filesystem scanner from `Grov-Original/src/integrations/mcp/capture/antigravity-*.ts`, ported.

A future ADR may switch to the official SDK once it is 1.0 and the protocol surface we use is stable.

## Options considered

- **Hand-rolled JSON-RPC + polling capture.** What we picked.
- **Use the Go SDK (when stable).** The right answer at some future point. Today it churns; pinning to a pre-1.0 version means re-pinning every release. Reject for v1.
- **Re-introduce hook-based capture for Claude Code.** Was tried, removed. The decision to remove is captured in `docs/SESSION_DEC2_2025_HOOKS_REMOVAL.md`. Reject.
- **Custom protocol over websockets / unix socket.** Breaks the MCP-spec compliance constraint. Reject.

## Consequences

Easier:

- We control the wire surface end-to-end. No surprises from SDK upgrades.
- Hook-free means one fewer integration to maintain per IDE; new IDEs need only a polling target, not a hook contract.
- Capture is uniform across IDE/CLI/Antigravity; one watcher abstraction parameterised by source.

Harder:

- A real MCP spec extension (resources, prompts, sampling) costs more code than `Tool.List/Tool.Call` did. We add only what users need; if the spec moves and we move with it, the cost is real.
- Hand-rolled means we own protocol bugs. Mitigated by a small test suite that boots the server with stdin from a fixture and asserts byte-equal replies.
- Polling has a 5-second-ish latency between "user finished a session" and "task is in the DB". Users on the proxy path have lower latency. Documented; not changing without a real complaint.
