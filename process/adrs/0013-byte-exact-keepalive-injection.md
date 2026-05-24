---
number: 0013
title: Extended-cache keep-alive uses byte-offset injection on the messages array
date: 2026-05-24
status: accepted
brief: 009-extended-cache
supersedes: null
superseded-by: null
---

# 0013 — Extended-cache keep-alive uses byte-offset injection on the messages array

## Context

Brief 009 explains the feature; ADR 0006 explains the byte-exact discipline. This ADR pins the keep-alive's specific edit shape so it cannot be casually "improved".

The TS code, at `Grov-Original/src/integrations/proxy/cache/extended-cache.ts`:

1. Stores the most recent forwarded request's raw bytes plus its safe headers in an in-memory map keyed by session id.
2. On each 60-second tick, for entries past the 4-minute idle threshold and below the per-period keep-alive cap (2):
   a. Finds the closing `]` of the `messages` array in the raw bytes.
   b. Splices in `,{"role":"user","content":"."}` immediately before that `]`.
   c. Validates the resulting buffer is still parseable JSON (sanity check, not a re-marshal).
   d. Forwards the modified buffer to Anthropic with the stored safe headers.
   e. Discards the response (the model's `.` reply is not part of the conversation).
3. Drops entries that have exceeded 10-minute total idle.
4. Clears all entries on `SIGINT` / `SIGTERM`.

The TS source contains explicit `// CRITICAL: do NOT modify max_tokens or stream` comments. Their position before the `messages` array means changing them breaks the cache prefix. The ADR exists so a Go reader sees it, not just a comment in a buffer they may not be looking at.

## Decision

The Go keep-alive uses the same byte-offset edit. Specifically:

- Locate the `messages` array's value (`messages` key, then the matching open `[`) by an unbalanced-bracket scan that treats string regions as opaque (handles escaped quotes, unicode escapes).
- Locate the matching closing `]`.
- Splice `,{"role":"user","content":"."}` immediately before the closing `]`.
- Run `json.Valid` on the resulting buffer; on `false`, abort the keep-alive (do not retry; the next tick may catch it).
- Forward via the same vendor `Wire.Forward(...)` used by the user's request path; the Anthropic adapter is the only adapter that supports this operation in v1.

We do NOT touch any field outside the `messages` array. We do NOT add fancier dedupe (e.g. detecting "we already sent a `.`"); two consecutive `.` user messages are a non-issue at the model-output level since responses are discarded.

The constants are: idle threshold `4 minutes`, per-period max keep-alives `2`, total max idle `10 minutes`, tick interval `60 seconds`. Configurable only via the on/off env var (`KETE_EXTENDED_CACHE`); the constants are not user-facing knobs.

## Options considered

- **Byte-offset injection on the messages array, fixed constants.** What we picked. Mirrors TS.
- **`json.Marshal` the modified body.** Breaks the cache prefix. Reject.
- **A "safe" library (`github.com/tidwall/sjson`) that does in-place edits.** Tempting; sjson canonicalises whitespace in some operations, which we cannot afford. Reject.
- **Configurable timing constants.** Would invite users to tune for "more keep-alives = always cached". The cost model only works at the TS-chosen cadence. Lock the constants until evidence says otherwise. Reject.

## Consequences

Easier:

- The keep-alive is a small focused module (~120 LOC) that reuses the `RawBody` helpers from ADR 0006.
- Cache-hit math is verifiable against the TS implementation by comparing produced request bytes on the same fixture.
- The "do not modify max_tokens or stream" rule has a reason captured in this ADR, not just a comment.

Harder:

- The bracket scanner must correctly handle JSON string syntax, including escapes. We cover with a fuzz test that feeds randomly mutated valid JSON and asserts the scanner never reports an offset inside a string.
- Future Anthropic API changes (e.g. moving `cache_control` semantics) could change what constitutes "the cached prefix". We will revisit if Anthropic announces a relevant change; we do not preempt.
