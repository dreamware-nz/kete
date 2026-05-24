# CLI reference

> Generated from `kete --help`. Run `make docs` to regenerate.

## `kete`

Local memory and reasoning layer for AI coding sessions

```
kete
```

### `kete doctor`

Diagnose kete setup

```
kete doctor
```

### `kete drift-test`

Score drift on a prompt or fixture set

Score drift on a single prompt (with --goal) or run an entire
fixture file with --fixture <path>. Fixture mode prints a per-row
table comparing expected vs actual level so you can eyeball
calibration against the hand-labelled set in testdata/drift/.

```
kete drift-test [<prompt>] [flags]
```

Flags:

```
      --fixture string   path to a fixture JSON file
      --goal string      stated goal of the session

```

### `kete mcp`

Run the stdio MCP server

```
kete mcp
```

### `kete proxy`

Run the local HTTP proxy

```
kete proxy [flags]
```

Flags:

```
      --debug            enable verbose request/response logging
      --extended-cache   extend Anthropic prompt cache via keep-alive injection

```

### `kete purge`

Delete the kete dotdir

```
kete purge [flags]
```

Flags:

```
      --yes   skip confirmation

```

### `kete status`

Show captured tasks for the current project

```
kete status [flags]
```

Flags:

```
      --all   list tasks across all projects

```

### `kete tasks`

Search captured tasks by goal/keywords

```
kete tasks <query>
```

