---
number: 0007
title: Split the agent adapter into wire and semantic interfaces
date: 2026-05-24
status: accepted
brief: 002-local-proxy
supersedes: null
superseded-by: null
---

# 0007 — Split the agent adapter into wire and semantic interfaces

## Context

The TS implementation has a single `BaseAdapter` abstract class with around 22 abstract methods covering: HTTP forwarding, body parsing, project-path extraction, session-id extraction, text-content extraction, goal extraction, history extraction, usage extraction, response validation, sub-agent detection, end-of-turn detection, tool-use detection, action parsing, tool-use block listing, internal-tool finding, memory injection, delta injection, tool injection, raw-body system-prompt injection, raw-body user-message injection, raw-body tool injection, response-header filtering, continue-body building, settings.

That's two responsibilities tangled together. **Wire** ops (parse this body, forward this request, pull these fields out, filter these headers, edit this raw byte buffer) are the bottom half; they're per-vendor (Anthropic vs OpenAI). **Semantic** ops (extract the user's goal, decide if this is end-of-turn, build a continue-body for the `kete_expand` tool loop, decide if it's a sub-agent) are the top half; some are per-vendor, some are not.

A 22-method interface in Go is the kind of thing that breeds shallow `func (a *anthropicAdapter) ExtractTextContent(...)` indirection wrappers nobody reads. Better to split.

## Decision

We split the adapter into two interfaces:

- **`Wire`** — vendor-specific HTTP and byte-level operations.
  - `Forward(ctx, raw RawBody, headers Headers) (Response, error)`
  - `ParseRequest(raw RawBody) (RequestView, error)` — typed read-only view
  - `ParseResponse(stream io.Reader) iter.Seq[ResponseEvent]` — streaming iterator
  - `Inject(raw RawBody, ops ...InjectionOp) error`
  - `FilterHeaders(in http.Header) http.Header`

- **`Semantics`** — operations that read or write the typed view, mostly vendor-portable.
  - `Goal(req RequestView) string`
  - `History(req RequestView) []Message`
  - `Usage(resp ResponseView) Usage`
  - `IsEndTurn(resp ResponseView) bool`
  - `IsToolUse(resp ResponseView) bool`
  - `Actions(resp ResponseView) []Action`
  - `BuildContinueBody(req RequestView, resp ResponseView, toolID string, toolResult string) (RawBody, error)`

Vendor packages (`internal/proxy/anthropic`, `internal/proxy/openai`) implement both interfaces and expose a single `Adapter` value that embeds both. The orchestrator depends on the interfaces, not the concrete types.

## Options considered

- **Two interfaces (Wire, Semantics).** What we picked. Lets the orchestrator depend on what it actually needs, makes the per-vendor file structure honest (one wire, one semantics), and cuts the number of methods the orchestrator has to mock in tests.
- **One mega-interface, mirroring TS.** Lossless port but smells. Reject.
- **A `Vendor` enum + giant switch.** Reads cleanly until the second vendor; falls apart on the third (Codex / Anthropic / hypothetical Gemini). Reject.

## Consequences

Easier:

- Orchestrator tests mock `Semantics` with a small fake, pass a real `Wire` against `httptest.NewServer`. Two well-scoped surfaces are clearer than one big one.
- Adding a new vendor is "implement two interfaces", and the second one is mostly shared utility code parameterised by a vendor-specific event format.
- The "is this Anthropic or OpenAI" question is asked once in the router; nothing downstream re-asks it.

Harder:

- Two interfaces means two implementations to find when reading. We mitigate by colocating: `internal/proxy/anthropic/wire.go` and `…/semantics.go` next to each other.
- Some methods could live in either interface. We pick once per method based on "does this need byte-level access or just typed view"; we don't relitigate.

If a third vendor lands and the boundary isn't right, that's a new ADR (super- or partial-supersede). Not a refactor we do preemptively.
