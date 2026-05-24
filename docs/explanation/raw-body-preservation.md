# Why we preserve raw request bodies

> Short version: **`json.Marshal` of a parsed body is not the same
> bytes as what came in.** Different bytes mean a different cache
> prefix, which means a cache miss, which means money.

ADR 0006 is the load-bearing rule. This doc explains why.

## Anthropic's prompt cache

Anthropic caches request prefixes. If two requests share the same
opening bytes — up to a `cache_control` breakpoint — the second one
hits a cached `cache_creation` charge (`1.25×` base price for the
prefix, counted once) and a `cache_read` charge (`0.1×`) for using
it. Miss the prefix and you pay full `cache_creation` again, plus
the prefix counts against your ITPM rate limit.

The match is **byte-exact**. Two requests that decode to the same
JSON object but serialise differently produce different ETags. The
cache misses.

## Where re-marshalling sneaks in

Go's `encoding/json` is hostile here:

- **Map iteration order is sorted alphabetically.** A `map[string]any`
  round-trip changes key order to `a, b, c…` regardless of input.
- **Struct fields serialise in struct definition order.** Different
  from map order, and different from input.
- **Number formatting may shift.** `1.0` may become `1`, or vice
  versa, depending on the type.
- **Escape style.** `<`, `>`, `&` get `\u003c`-style escapes by
  default unless you ask `Encoder.SetEscapeHTML(false)`.
- **Whitespace.** Indentation, newlines, key separators all change.

A request that came in as
`{"messages":[…],"model":"claude","max_tokens":4096}` and round-trips
through `json.Marshal(map[string]any{…})` comes out as
`{"max_tokens":4096,"messages":[…],"model":"claude"}` — different
bytes, different cache prefix, miss.

## Our discipline

The proxy treats the request body as `[]byte`. We pass the same
`[]byte` to the upstream. We never call `json.Marshal` on a parsed
body in the forward path.

For inspection (memory injection ranking, drift scoring, upstream
selection) we `json.Unmarshal` into a typed view, use it read-only,
and discard. The typed view never goes back to the wire.

For mutation (memory injection, correction injection, keep-alive
splice) we operate on the `[]byte` with byte-offset edits via
`internal/inject`, validate the result still parses, and forward.

```go
// good:
out, err := inject.BeforeCacheBreakpoint(rawBody, payload)

// bad:
var v map[string]any
json.Unmarshal(rawBody, &v)
v["messages"] = appendSomething(v["messages"])
out, _ := json.Marshal(v)   // wrong bytes, cache miss
```

## The deliberate exception: Bedrock

Bedrock's invoke endpoint is structurally different — model id in
the URL, `anthropic_version` in the body, no `model` field — so
the bytes have to change. ADR 0014 captures this. We accept the
re-marshal because Bedrock has its own prompt cache anyway and the
keying is its own.

The other deliberate exception is auto-compaction (ADR none, plan
008): when the context window fills, we drop the prior conversation
and rewrite `messages` with a structured summary plus the new
prompt. The cache prefix breaks by design — we are starting a new
prefix from a clean body.

## Why this rule lives in code-review territory

The temptation to refactor a byte-slice scanner into a typed
`json.Marshal(View)` round-trip is high. Future contributors will
read `inject.AtMessages` and want to "simplify" it. **Don't.** The
`internal/inject` package's tests assert prefix-byte stability:

```go
once, _ := inject.BeforeCacheBreakpoint(in, payload1)
twice, _ := inject.BeforeCacheBreakpoint(in, payload2)
// bytes before the injected segment must be identical:
require.True(t, bytes.Equal(once[:n], twice[:n]))
```

Removing those tests is the loud signal that the rule is being
broken. They've been there since plan 002.

## Validation

`internal/inject` runs `json.Valid(out)` after every splice. If a
mutation produces invalid JSON we return the original bytes and
surface the error. Garbage in, garbage out is acceptable; invalid
JSON to the upstream is not.

## See also

- ADR 0006 — the rule.
- ADR 0014 — the Bedrock exception.
- `internal/inject/inject.go` — the helpers.
- `internal/inject/inject_test.go` — the prefix-stability tests.
