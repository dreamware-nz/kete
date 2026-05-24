# First run

> Five minutes from `git clone` to a captured Crush task.

## 1. Build

```sh
git clone git@github.com:dreamware-nz/kete.git
cd kete
make build
```

The binary lands at `bin/kete`.

## 2. Sanity-check the install

```sh
./bin/kete doctor
```

You should see:

```
PASS  dotdir    /Users/you/.kete
PASS  upstream  https://api.anthropic.com — HTTP 404
```

The `404` is fine — the upstream is reachable; the path probe lands
on a non-existent endpoint, which means it's responding.

## 3. Start the proxy

```sh
export ANTHROPIC_API_KEY=sk-...   # your direct-API key
./bin/kete proxy &
```

Default bind is `127.0.0.1:8080`.

## 4. Point Crush at it

In your Crush config, replace the Anthropic base URL:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:8080
```

Run a normal Crush session. Drive a small task — refactor a function,
add a test, anything that's a few prompts long.

## 5. Confirm capture

```sh
./bin/kete status
```

You should see one or more rows, with the cwd as the project path.

```sh
./bin/kete tasks "<keyword from your task>"
```

returns the captured trace.

## What just happened

- `kete proxy` ran on `127.0.0.1:8080`.
- Crush's `POST /v1/messages` requests went through it byte-exact
  (so the prompt cache still works).
- After each request, kete asynchronously wrote a `tasks` row to
  `~/.kete/memory.db`.
- On every fresh prompt, kete re-reads recent tasks for the cwd and
  splices them into the request body before forwarding.

## Where to next

- **Use cc-proxy (Claude Code subscription)** — see
  `docs/how-to/use-cc-proxy.md` (when that lands).
- **Use Bedrock** — see `docs/how-to/use-bedrock.md` (when that lands).
- **Add the MCP server** — see `docs/reference/mcp.md`. Run kete as
  a stdio MCP server so Crush can call `kete_preview` / `kete_expand`
  directly.
- **Tune drift detection** — `kete drift-test "<prompt>" --goal "..."`.
