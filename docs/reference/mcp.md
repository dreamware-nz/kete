# MCP reference

> stdio JSON-RPC 2.0 MCP server. Newline-delimited messages on stdin/
> stdout (per ADR 0012). Logs go to `~/.kete/kete-mcp.log`; nothing
> ever lands on stdout/stderr that isn't an MCP frame.

Run with:

```sh
kete mcp
```

A typical Crush MCP config entry:

```json
{
  "mcpServers": {
    "kete": { "command": "kete", "args": ["mcp"] }
  }
}
```

## Tools

### `kete_preview`

Return up to 3 candidate prior-task previews for the current prompt.

Input schema:

```json
{
  "type": "object",
  "properties": {
    "context": {"type": "string"},
    "mode":    {"type": "string", "enum": ["project", "all"]}
  },
  "required": ["context"]
}
```

Output (as a single `text` content block, JSON-encoded):

```json
{
  "previews": [
    {
      "id": "a3f1b2c4",
      "summary": "refactor auth flow",
      "files_touched": ["internal/auth/login.go"],
      "created_at": "2026-05-24T07:29:31Z"
    }
  ]
}
```

The 8-character `id` is a process-lifetime handle into the server's
preview cache. Pass it to `kete_expand`.

### `kete_expand`

Return the full reasoning trace for one previewed task.

Input schema:

```json
{
  "type": "object",
  "properties": {
    "id": {"type": "string"}
  },
  "required": ["id"]
}
```

Output:

```json
{
  "goal": "refactor auth flow",
  "decisions": [{"choice": "...", "rationale": "..."}],
  "files_touched": ["internal/auth/login.go"],
  "reasoning_trace": "...",
  "created_at": "2026-05-24T07:29:31Z"
}
```

If the id wasn't issued by this server process, the call returns a
tool-level error (`isError: true`) with the message
`unknown id; call kete_preview first this session`.

## Wire example

```text
→ {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"crush","version":"x"}}}
← {"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"kete","version":"0.1.0"}}}

→ {"jsonrpc":"2.0","id":2,"method":"tools/list"}
← {"jsonrpc":"2.0","id":2,"result":{"tools":[{...kete_preview...},{...kete_expand...}]}}

→ {"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"kete_preview","arguments":{"context":"auth flow"}}}
← {"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"{\"previews\":[...]}"}]}}
```

## Why two tools and not one

See `docs/explanation/why-proxy-not-just-mcp.md`. The short version:
returning full memories on every preview wastes tokens; returning only
ids forces the model to commit blindly. Preview-then-expand is the
honest middle path.
