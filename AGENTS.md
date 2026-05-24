# AGENTS.md

Guide for agents working in `kete`. Read `README.md` and `process/PROJECT.md`
for what the project *is*; this file covers what's non-obvious about
*changing it*.

`CLAUDE.md` and `CRUSH.md` are gitignored — leave per-agent notes there
if you want them per-clone, but don't commit them.

## Commands

| What                    | How                                                                |
| ----------------------- | ------------------------------------------------------------------ |
| Build                   | `make build` → `bin/kete`                                          |
| Test                    | `make test` (= `go test ./...`); CI runs `go test -race -count=1`  |
| Format check            | `gofmt -l .` must be empty                                         |
| Tidy check              | `go mod tidy` must produce no diff in `go.mod` / `go.sum`          |
| Vet                     | `go vet ./...`                                                     |
| Regenerate CLI ref      | `make docs` → `docs/reference/cli.md` (CI fails if drifted)        |
| Cross-build releases    | `make release` → `dist/` (darwin+linux × amd64+arm64, sha256 file) |
| Single test             | `go test ./internal/proxy -run TestX -v`                           |

CI matrix: ubuntu + macos. Lint job is ubuntu-only. Docs job is ubuntu-only
and **will fail the build if `docs/reference/cli.md` is stale** — re-run
`make docs` after any change to a cobra command's `Use` / `Short` / `Long`
/ flags.

`CGO_ENABLED=0` is mandatory in release builds (ADR 0002, pure-Go SQLite
via `modernc.org/sqlite`). Don't add a cgo dependency without an ADR.

## Layout

```
cmd/kete/        binary entry point (one-line shim into internal/cli)
cmd/ketedoc/     doc generator that walks the cobra tree → markdown
internal/cli/    cobra command tree (proxy, mcp, status, tasks, drift-test, doctor, purge)
internal/proxy/  HTTP server: routing, header sanitisation, capture, drift, compact, expand-loop, usage tap
internal/mcp/    stdio JSON-RPC 2.0 server, tools: kete_preview, kete_expand
internal/adapter/{anthropic,bedrock,ccproxy}/  per-upstream Wire impls
internal/inject/ raw-byte JSON splicing (AtMessages, BeforeCacheBreakpoint, …)
internal/extract/ Haiku-backed extraction client + embedded prompts
internal/store/  SQLite, embedded migrations, typed CRUD
internal/drift/  scoring, level-mapping, persistence, fixture loader
internal/compact/ summary build + Apply that rewrites `messages`
internal/keepalive/ extended-cache opt-in (ADR 0013)
internal/capture/ project-path normalisation; helpers shared across capture sources
process/         briefs, ADRs, plans, designs (read before changing shapes)
docs/            Diátaxis-shaped user docs
testdata/drift/  hand-labelled drift fixtures
contrib/launchd/ launchd template + install script
```

## Process

The repo follows `idea → brief → ADR(s) → plan (phased) → execute`.
Templates live in `process/templates/`. Two operational rules:

- **ADRs are immutable once accepted.** Don't edit `process/adrs/0006-*.md`.
  Supersede with a new ADR.
- **Decision drift goes in `process/drift.md`** — one line per drift, dated.
  The next `process-retrospect` pass turns drifts into ADR / brief revisions.
- The PR template wants a `Decision:` git trailer for non-trivial choices
  (added dep, public-API change, viable-alternative pick) and a link to
  the ADR or `drift.md` line.

The `process-*` skills (in `~/.claude/skills/`) drive each stage. ADR
0006 (byte-exact bodies) and ADR 0015 (three-upstream selection) are
the two most-frequently-load-bearing for proxy work.

## The byte-exact rule (ADR 0006)

**The single most load-bearing constraint in the codebase.** Anthropic's
prompt cache matches by byte-exact prefix; `json.Marshal` on a parsed
body alphabetises map keys and re-encodes numbers, which kills the
cache.

Rules:

- Read raw bodies as `[]byte`. Forward the **same** `[]byte` upstream.
- For inspection: `json.Unmarshal` into a typed view, **read-only**, discard.
- For mutation: byte-offset edits via `internal/inject/`. Each helper
  validates the result still parses with `json.Valid`.
- Never touch `max_tokens` or `stream` — they live before the cache
  breakpoint.
- Memory injection tries `BeforeCacheBreakpoint` first, falls back to
  `AtMessages`. The order matters: splicing *before* the cache marker
  preserves the prefix; splicing inside `messages` after the marker
  invalidates it.

The **sole exception** is `internal/adapter/bedrock/bedrock.go` — Bedrock
needs a body re-shape (drop `model`, set `anthropic_version` field,
re-marshal). ADR 0014 documents why and reaffirms ADR 0006 for everything
else.

If a future PR re-marshals JSON anywhere on the request path, that's a
red flag. Push back with ADR 0006.

## Three-upstream selection (ADR 0015)

`internal/proxy/route.go::SelectUpstream` applies precedence:

1. `X-Kete-Upstream` header (consumed and stripped before forwarding)
2. Model-id pattern (`arn:aws:bedrock:`, `us.anthropic.`, `anthropic.claude` → bedrock)
3. `KETE_UPSTREAM` env var
4. Default: `anthropic`

**`cc-proxy` cannot be auto-detected from the model id** because it uses
the same Anthropic ids as anthropic-direct. Selection must come from
header or env.

Adapters that fail to construct (no `AWS_REGION`, no `KETE_CC_PROXY_KEY`)
are left `nil` in the adapter map. Hitting a nil adapter returns 501,
not 500 — keep it that way so the user gets a clear error.

## Header handling (`internal/proxy/headers.go`)

`SanitiseHeaders` is a strict **allow-list**, not a deny-list. Adding a
new end-to-end header (e.g. a new `anthropic-beta` flavour) means
editing `forwardedHeaders`. Keep `secretHeaders` in sync if the new
header carries auth.

Logging must use `RedactForLog` for any header dump — `x-api-key` and
`authorization` get `[REDACTED]`, and AWS-credential-shaped values are
redacted as belt-and-braces.

## Storage

- Default DB: `~/.kete/memory.db` (`KETE_DB_PATH` overrides; `KETE_HOME`
  overrides the dir). Dir 0700, file 0600 — enforced in
  `internal/store/path.go`. Don't widen the perms.
- **`SetMaxOpenConns(1)`** is intentional. SQLite serialises writes
  through one writer lock; a larger pool produces `SQLITE_BUSY` under
  the proxy's concurrent capture/enrich/drift writers. Don't bump it
  without an ADR.
- Migrations: numbered, up-only, embedded via `//go:embed
  migrations/*.sql`. Add a new file `0006_*.sql`; never edit an applied
  one.
- `Close()` runs a `wal_checkpoint(TRUNCATE)` so on-disk artefact is
  just `memory.db` with no `-wal` / `-shm` hangers-on.

## Project key

Every capture / injection is keyed by *project path*. Always resolve via
`capture.NormaliseProject` so symlinked paths collapse to one identity.
Both `internal/proxy/project.go` (`projectPath()`) and
`internal/capture/capture.go` honour `KETE_PROJECT` first, else cwd,
both with `EvalSymlinks`. If you add a third caller, route it through
`capture.NormaliseProject` — don't reimplement.

## Request lifecycle (POST /v1/messages)

`internal/proxy/server.go::handleMessages`:

1. Read body (capped by `maxBodyMiddleware`; 413 on overflow).
2. `SelectUpstream(headers, body)`.
3. If a prior turn crossed the compact-clear threshold, rewrite body via
   `compact.Apply` (this is a deliberate re-marshal — ADR 0006 exception
   for compaction).
4. `injectMemory` — splice up to 3 prior tasks. Failure is non-fatal
   (logs to stderr; brief 002 says enrichment never blocks forwarding).
5. Drain pending drift correction (one per request).
6. Wrap writer in a `usageTap` that side-bands Anthropic SSE
   `message_start` / `message_delta` token counts to the compact hook.
7. **Streaming requests** pass straight through (Crush handles
   `kete_expand` client-side via stdio MCP).
   **Non-streaming requests** run the **expand-loop** (`runExpandLoop`)
   which inspects responses for `kete_expand` tool_use blocks, resolves
   them locally, builds a continue-body, re-forwards. Capped at
   `maxExpandCycles` (5).
8. Capture runs **on the pre-injection body** so kete's own injections
   aren't recaptured — that's a load-bearing detail; don't refactor
   capture to read `injected`.

Capture / drift / compact-summary work runs in goroutines via
`s.capture.Wait()` and friends — `shutdown()` waits on them with a
500ms grace.

## Extraction (Haiku)

`internal/extract/client.go` always points at Anthropic-direct, regardless
of which upstream the user chose. Reasoning: extraction is kete's own
billing problem, and Bedrock / cc-proxy don't host Haiku-class models
under stable ids. Needs `ANTHROPIC_API_KEY` (or `KETE_ANTHROPIC_URL`
pointed at a relay). Default model is `claude-haiku-4-5-20251001`;
`KETE_DRIFT_MODEL` overrides.

Prompts are plain `.txt` files in `internal/extract/prompts/` and the
two MCP tool descriptions are in `internal/mcp/tools/*.txt`. Edit the
text files directly; they're loaded via `embed` at compile time. ADR
0008 says tool descriptions are kete-authored, not generated.

## Drift

Score 0–10 → four levels (`none` / `nudge` / `correct` / `intervene` /
`halt`). State machine and persistence in `internal/drift/`. Drift fires
every Nth prompt (`KETE_DRIFT_CHECK_INTERVAL`, default 5). The corrected
text is queued on the server's `corrections` map and consumed by the
**next** request (one correction per request).

Calibration is open: `kete drift-test --fixture testdata/drift/fixtures.json`
reports per-row expected-vs-actual. Live runs against Haiku 4.5 currently
match ~7/24 — model is harsher than fixture labels. Tuning prompts /
thresholds is its own brief; don't fold it into unrelated PRs.

## MCP server

Hand-rolled JSON-RPC 2.0 over stdio (ADR 0012). **stdout/stderr belong
to the protocol** — logs go to a file under `~/.kete/`, never to either.
If you add a `fmt.Println` while debugging, you'll corrupt the wire.
Tools: `kete_preview` (cheap one-liner previews from sqlite), `kete_expand`
(full reasoning trace by 8-char id). Process-lifetime ids; not durable
across MCP sessions.

## Testing patterns

- Tests use `t.TempDir()` + `KETE_HOME` to isolate the dotdir per-test.
- Proxy integration tests use `httptest.NewServer` as the upstream and
  point the adapter's `BaseURL` at it.
- Streaming tests build SSE bodies inline; check usage-tap parsing
  against `event: message_start` and `event: message_delta` shapes.
- Race detector is mandatory on CI (`-race`); the proxy spins up
  capture / enrich / drift goroutines so race-free is non-negotiable.
- Drift fixtures (`testdata/drift/fixtures.json`) are hand-labelled;
  don't auto-generate.

## Style

- `gofmt`, no exceptions.
- Doc comments on every exported identifier, in package-prose form
  (look at `internal/store/store.go`, `internal/adapter/adapter.go` for
  the house style — declarative, one-paragraph context, sometimes a
  rationale link to an ADR).
- Comments explain *why*, not *what*. ADR references are the most
  common form (`ADR 0006`, `brief 002`, `plan 002 phase 16`).
- Prefer raw-byte ops over typed round-trips on the request path
  (ADR 0006).
- Errors are values; let them propagate. Don't wrap defensively. The
  proxy's enrichment paths log-and-continue on purpose — failure is
  non-fatal by design.

## Common gotchas

- **Don't re-marshal request JSON.** ADR 0006. Bedrock is the only
  exception.
- **Don't print to stdout/stderr from `kete mcp`.** It's the JSON-RPC
  wire.
- **`make docs` drift breaks CI** — regenerate after touching cobra
  command help.
- **`go mod tidy` drift breaks CI** — run after adding/removing a dep.
- **`gofmt` drift breaks CI** — run before committing.
- **Don't bump `SetMaxOpenConns`** — SQLite write serialisation depends
  on it.
- **Don't widen `~/.kete` perms** — 0700 / 0600 is enforced.
- **`KETE_UPSTREAM=cc-proxy` cannot be inferred from model id** — must
  come from header or env.
- **Capture reads the pre-injection body**, deliberately. Don't change
  it to read the injected one.
- **Memory injection picks the splice point based on `cache_control`**:
  `BeforeCacheBreakpoint` first, `AtMessages` fallback. Reordering
  changes cache-hit behaviour silently.
- **The expand-loop runs only for non-streaming requests.** Crush
  handles `kete_expand` for streaming via the stdio MCP server.
- **Adapters that fail to construct return 501**, not 500, when
  selected. Preserve that.
- **`internal/seedutil/`** is currently empty — placeholder for future
  seed helpers; don't be surprised it has no `.go` files.

## Live verification

End-to-end smokes in CHANGELOG 0.1.0 went through real AWS Bedrock +
Anthropic Haiku 4.5 (`us.anthropic.claude-haiku-4-5-20251001`).
Anthropic-direct and cc-proxy paths are wire-shape tested but not live-
smoked in CI (no API key in test env). If you change adapter wire
behaviour, smoke against a real upstream and note the model / region /
account in the PR.

## Install / runtime layout

- Binary: `~/.local/bin/kete` (or `$PREFIX/bin/kete`).
- Data: `~/.kete/` (dir 0700) — `memory.db` (0600), logs, optional
  cached state.
- launchd template: `contrib/launchd/kete.proxy.plist.template` plus an
  installer script. See `docs/how-to/run-as-a-service.md`.
- Override env vars listed in `docs/reference/env.md` — that file is
  the source of truth for env vars.

## Where to read next

- ADRs in `process/adrs/` — start with 0000 (identity), 0006 (byte-exact),
  0007 (Wire/Semantics split), 0014 (Bedrock exception), 0015
  (three-upstream selection).
- `docs/explanation/raw-body-preservation.md` — narrative version of
  ADR 0006.
- `docs/explanation/three-upstreams.md` — narrative version of ADR 0015.
- Master plan: `process/plans/000-kete-overview.md`.
