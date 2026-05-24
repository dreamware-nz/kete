---
id: 003-mcp-server
date: 2026-05-24
status: shipped
from-idea: 2026-05-24-kete
design: null
adrs: [0012-mcp-stdio-jsonrpc-library, 0008-mcp-tools-and-descriptions]
plan: 003-mcp-server
---

# 003 — MCP server (complementary to the proxy)

## Problem

The proxy (brief 002) handles capture, injection, drift, and compaction by sitting in the request stream. There is also a value to letting the model *participate* in retrieval: when the model is good at tool-use, it can ask "is there prior reasoning relevant to this prompt?" and pull only what it actually needs, rather than receiving everything the proxy injected.

That's what the MCP layer is for. kete ships a stdio MCP server with two tools — `kete_preview` (return up to 3 previews of relevant memories for a context string) and `kete_expand` (return the full reasoning trace for a previewed memory by 8-character ID). Both tools read from the same local store the proxy reads from; neither writes.

This is **complementary, not primary.** The proxy injects unconditionally so capture/inject works on any model. The MCP tools let *capable* models do better — fetch on demand, expand when interested, stop when satisfied — saving tokens by not injecting everything up-front. If the model ignores the tools entirely, the proxy still did its job. If the proxy is bypassed (e.g. a future client that doesn't proxy through us), the MCP tools provide a fallback retrieval path.

## Why two tools

`kete_preview` then `kete_expand` is the same shape grov landed on after iteration. The reasoning is honest: returning full memories on every preview wastes tokens; returning only IDs forces the model to guess. A small candidate-set with summaries gives the model enough information to commit to one. Two tools, one cycle.

We write our own tool descriptions (ADR 0008), tuned for the models Crush actually uses. The descriptions are part of the wire surface — changing them is an ADR-level decision — but they are *our* surface, not borrowed.

## Who is hurt

- **Capable models on Crush sessions** that would benefit from expanding a single memory rather than receiving three full memories injected by the proxy. Without the MCP tools, they have no opt-in fetching.
- **Future non-proxy clients** (if any). The proxy is the primary path; MCP is the fallback that still gets capture/injection benefits without sitting in the request stream.

## Constraints

- **Stdio JSON-RPC 2.0**, MCP-spec compliant. Hand-rolled per ADR 0012.
- **Two tools only.** `kete_preview(context: string, mode: string)` and `kete_expand(id: string)`. Adding tools is an ADR-level decision.
- **Tool descriptions are part of the wire surface.** They live in `internal/mcp/tools/preview.txt` and `expand.txt`, loaded via `go:embed`. Editing them is an ADR-level decision (ADR 0008).
- **Project-path resolution must agree with the proxy.** Both layers must produce the same `project_path` key for the same Crush session, or memories saved by the proxy aren't visible to MCP and vice versa. Algorithm: `KETE_PROJECT_PATH` env override → folder name (not full path) of `WORKSPACE_FOLDER_PATHS[0]` → `cwd`. Same as the proxy.
- **Logs go to `~/.kete/kete-mcp.log`.** Never to stdout/stderr from the request path — stdio is the wire.
- **In-memory 8-char ID cache** keyed per-session. Lifetime: one preview→expand cycle.
- **No capture from the MCP server.** MCP is read-only. All writes go through the proxy.

## Success looks like

- A Crush session through the proxy completes a task; the proxy injects prior reasoning into the first request; partway through, the model calls `kete_preview` with a related question and gets three new candidate memories; it calls `kete_expand` on one and gets the full trace; the response references both the proxy-injected and the MCP-fetched memories.
- A regression test boots the MCP server with a fixture stdin, asserts tool-list and tool-call responses match a captured baseline.
- `KETE_PROJECT_PATH` set forces both the proxy and the MCP server to use the same project key.

## Non-goals

- New MCP tools. Two is the contract.
- HTTP / WebSocket MCP transport. Stdio only.
- A general MCP framework. We ship one server.
- Authoring MCP-spec features beyond `tools/list` and `tools/call`. Resources, prompts, sampling — none of them ship in v1.
- Capture via MCP. The proxy is the only capture path.
- Multi-client compatibility surface beyond what the MCP spec already requires. Crush is the user.

## Open questions

- `[adr]` 0014 — already settled: hand-rolled JSON-RPC.
- `[adr]` 0008 — own the tool descriptions. Settled.
- Whether `kete_preview`'s `mode` parameter is used. grov has it; we may not need it. Defer to plan stage; ship the parameter, ignore its value for v1.

## Doc impact

- `docs/reference/mcp.md` `[new]` — tool schemas, JSON shapes, env vars.
- `docs/how-to/setup-crush-mcp.md` `[new]` — Crush config snippet for kete as an MCP server.
- `docs/explanation/proxy-vs-mcp.md` `[new]` — when each layer activates and why both run.
