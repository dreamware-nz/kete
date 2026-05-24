# Why a proxy and not just MCP

> Short version: **the proxy is the only contract that's not
> cooperative.** Hooks rely on Crush. MCP tool calls rely on the
> model. The HTTP request rises above both.

## The three Crush surfaces

If you wanted kete-the-feature without owning a proxy, you'd reach
for one of two existing Crush surfaces:

1. **Crush hooks** — `SessionStart`, `Stop`, `UserPromptSubmit`, etc.
2. **MCP tools** — exposed via stdio, called by the model when it
   decides to.

Both are real. Both are *cooperative*. Compare:

| Surface           | Who decides whether it runs        | Failure mode                                          |
| ----------------- | ---------------------------------- | ----------------------------------------------------- |
| Crush hooks       | Crush, on lifecycle events Crush exposes | Crush adds a new event we can't see; or removes one we depended on |
| MCP tool call     | The model, when the model decides  | Cheap/distracted models routinely ignore "MANDATORY" tool descriptions |
| **HTTP request**  | **Nobody. The bytes have to flow somewhere.** | **None we can be locked out of**            |

Hooks are a contract between us and Crush. MCP tools are a contract
between us and the model. **Both can fail silently.** The proxy is a
contract between us and the wire — the request has to traverse it,
regardless of what Crush decided to surface or what the model decided
to call.

## Concretely

- **Capture** cannot rely on the model calling a tool. A Crush
  session on a weak model would never capture. The proxy sees every
  request.

- **Injection** benefits from being unconditional. The proxy splices
  prior reasoning into the request body before the model sees the
  prompt; the model can't "forget" to fetch context.

- **Drift correction** has to be inline. Once the agent has committed
  an action, our only remaining lever is to inject a correction *into
  the next request*, byte-mutating before forwarding. No hook gets us
  there. (See ADR 0011 for the four-level correction structure.)

- **Auto-compaction** has to rewrite the request. The conversation
  history *is* the request body. Compaction is a body rewrite, full
  stop.

## Why MCP still ships

The MCP server is **belt-and-braces**, not redundant. When the model
is smart enough to call `kete_expand` reliably, MCP complements the
proxy by letting the model fetch *full* reasoning on demand
(preview-then-expand, ADR 0008). When the model isn't, the proxy
carries the load alone.

Neither layer depends on the other. Both run.

## What the proxy gives up

- A network hop. Localhost, single-digit microseconds.
- One more daemon for the user to run. `kete proxy &`.
- Byte-discipline complexity in our codebase. ADR 0006 is the gate
  that keeps that under control.

## See also

- ADR 0006 — raw-body passthrough (the byte-exact rule).
- ADR 0007 — wire/semantic adapter split.
- ADR 0011 — drift correction: four levels.
- ADR 0015 — three upstreams, one selection rule.
- `docs/reference/proxy.md` — the operator surface.
- `docs/reference/mcp.md` — the cooperative surface.
