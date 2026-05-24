// Package inject contains byte-offset edits for kete's request bodies.
//
// All edits operate on raw []byte and preserve every byte before the
// edit point unchanged. ADR 0006 is the load-bearing constraint:
// json.Marshal'ing a parsed view loses the cache prefix.
//
// The helpers live in their own package so the proxy code can import
// them without circulars when extraction (plan 011) wants the same
// scanner.
package inject

import (
	"encoding/json"
	"errors"
	"fmt"
)

// AtMessages splices payload immediately before the closing `]` of the
// "messages" array, prefixed with a comma if the array is non-empty.
// payload must itself be a valid JSON object/array (no leading or
// trailing whitespace required).
//
// Example:
//
//	in:  {"messages":[{"role":"user","content":"a"}]}
//	out: {"messages":[{"role":"user","content":"a"},<payload>]}
//
// Empty messages array:
//
//	in:  {"messages":[]}
//	out: {"messages":[<payload>]}
func AtMessages(rawBody, payload []byte) ([]byte, error) {
	idx, err := findClosingBracket(rawBody, "messages")
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(rawBody)+len(payload)+1)
	prefix := rawBody[:idx]
	out = append(out, prefix...)
	if !arrayIsEmpty(prefix) {
		out = append(out, ',')
	}
	out = append(out, payload...)
	out = append(out, rawBody[idx:]...)
	if !json.Valid(out) {
		return nil, errors.New("inject: result is not valid JSON")
	}
	return out, nil
}

// BeforeCacheBreakpoint splices payload immediately before the first
// occurrence of `"cache_control"` in rawBody. The splice point lives
// inside whatever message/system block the cache_control marker is on,
// so the caller must hand a payload that's syntactically valid in that
// position (typically a `{...},` JSON object segment).
//
// If rawBody contains no `cache_control` marker, AtMessages is the
// right tool — this returns ErrNoCacheControl.
var ErrNoCacheControl = errors.New("inject: no cache_control marker found")

func BeforeCacheBreakpoint(rawBody, payload []byte) ([]byte, error) {
	idx, err := findCacheControlStart(rawBody)
	if err != nil {
		return nil, err
	}
	// Walk back to the start of the *containing* JSON object so we
	// splice before the whole {"role":..."cache_control":{...}} block,
	// not in the middle of a property.
	objStart := walkBackToObjectStart(rawBody, idx)
	out := make([]byte, 0, len(rawBody)+len(payload)+1)
	out = append(out, rawBody[:objStart]...)
	out = append(out, payload...)
	out = append(out, ',')
	out = append(out, rawBody[objStart:]...)
	if !json.Valid(out) {
		return nil, errors.New("inject: result is not valid JSON")
	}
	return out, nil
}

// findClosingBracket returns the offset of the matching `]` for the
// named top-level array field (e.g. "messages", "tools"). Top-level
// here means "directly under the root object".
//
// It is string-aware: brackets and braces inside JSON strings do not
// count toward depth, and escaped quotes inside strings do not close
// the string.
func findClosingBracket(b []byte, key string) (int, error) {
	keyBytes := []byte(`"` + key + `"`)
	keyOff := indexAtTopLevel(b, keyBytes)
	if keyOff < 0 {
		return -1, fmt.Errorf("inject: key %q not found at top level", key)
	}
	// Find the `[` that opens the array after `key:`.
	i := keyOff + len(keyBytes)
	for i < len(b) && b[i] != '[' {
		i++
	}
	if i >= len(b) {
		return -1, fmt.Errorf("inject: %q is not an array", key)
	}
	// Walk to matching `]`.
	depth := 0
	inString := false
	escape := false
	for j := i; j < len(b); j++ {
		c := b[j]
		switch {
		case escape:
			escape = false
		case c == '\\' && inString:
			escape = true
		case c == '"':
			inString = !inString
		case inString:
			// nothing
		case c == '[':
			depth++
		case c == ']':
			depth--
			if depth == 0 {
				return j, nil
			}
		}
	}
	return -1, fmt.Errorf("inject: unterminated array for %q", key)
}

// indexAtTopLevel finds needle in b only when it appears at depth 1
// (i.e. directly inside the root object). Strings/braces/brackets at
// depth > 1 are skipped.
func indexAtTopLevel(b, needle []byte) int {
	depth := 0
	inString := false
	escape := false
	for i := range len(b) {
		c := b[i]
		switch {
		case escape:
			escape = false
		case c == '\\' && inString:
			escape = true
		case c == '"':
			if !inString && depth == 1 && i+len(needle) <= len(b) {
				if equalBytes(b[i:i+len(needle)], needle) {
					return i
				}
			}
			inString = !inString
		case inString:
			// nothing
		case c == '{', c == '[':
			depth++
		case c == '}', c == ']':
			depth--
		}
	}
	return -1
}

// arrayIsEmpty reports whether the buffer up to (but not including)
// the closing `]` ends with `[` (allowing whitespace), i.e. the array
// has no elements yet.
func arrayIsEmpty(prefix []byte) bool {
	for i := len(prefix) - 1; i >= 0; i-- {
		switch prefix[i] {
		case ' ', '\n', '\t', '\r':
			continue
		case '[':
			return true
		default:
			return false
		}
	}
	return false
}

// findCacheControlStart returns the offset of the first occurrence of
// `"cache_control"` outside of a string. Returns ErrNoCacheControl if
// none.
func findCacheControlStart(b []byte) (int, error) {
	needle := []byte(`"cache_control"`)
	inString := false
	escape := false
	for i := range len(b) {
		c := b[i]
		switch {
		case escape:
			escape = false
		case c == '\\' && inString:
			escape = true
		case c == '"':
			if !inString && i+len(needle) <= len(b) && equalBytes(b[i:i+len(needle)], needle) {
				return i, nil
			}
			inString = !inString
		}
	}
	return -1, ErrNoCacheControl
}

// walkBackToObjectStart walks back from idx to the `{` that opens the
// containing JSON object. String-aware. The search is bounded by the
// last unmatched `[` we hit going backwards, which would mean we
// crossed an array boundary and the cache_control isn't inside an
// object after all.
func walkBackToObjectStart(b []byte, idx int) int {
	depth := 0
	inString := false
	for i := idx - 1; i >= 0; i-- {
		c := b[i]
		// Quotes are detected by a backward scan but escaping is
		// awkward; for our purposes (Anthropic-shaped bodies) the
		// cache_control marker is always on a fresh property, so the
		// simple "track depth" version is enough.
		switch {
		case c == '"':
			inString = !inString
		case inString:
			// nothing
		case c == '}':
			depth++
		case c == '{':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return 0
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
