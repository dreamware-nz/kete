package inject

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// sumOf returns sha256 of the byte prefix up to len(prefix), used to
// assert prefix bytes don't change across edits.
func sumOf(b []byte, n int) string {
	h := sha256.Sum256(b[:n])
	return hex.EncodeToString(h[:])
}

func TestAtMessages_NonEmpty(t *testing.T) {
	in := []byte(`{"model":"claude","messages":[{"role":"user","content":"a"}]}`)
	payload := []byte(`{"role":"assistant","content":"b"}`)
	out, err := AtMessages(in, payload)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"model":"claude","messages":[{"role":"user","content":"a"},{"role":"assistant","content":"b"}]}`
	if string(out) != want {
		t.Errorf("\n got %s\nwant %s", out, want)
	}
}

func TestAtMessages_Empty(t *testing.T) {
	in := []byte(`{"messages":[]}`)
	payload := []byte(`{"role":"user","content":"hi"}`)
	out, err := AtMessages(in, payload)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"messages":[{"role":"user","content":"hi"}]}`
	if string(out) != want {
		t.Errorf("\n got %s\nwant %s", out, want)
	}
}

// TestAtMessages_PreservesPrefix asserts the bytes before the
// "messages" key are identical to the input. This is the load-bearing
// invariant for cache-prefix preservation when messages live BEFORE
// any cache_control breakpoint.
func TestAtMessages_PreservesPrefix(t *testing.T) {
	in := []byte(`{"model":"claude","stream":true,"messages":[{"role":"user","content":"a"}]}`)
	prefixEnd := strings.Index(string(in), `"messages"`)
	wantSum := sumOf(in, prefixEnd)

	payload := []byte(`{"role":"assistant","content":"b"}`)
	out, err := AtMessages(in, payload)
	if err != nil {
		t.Fatal(err)
	}
	if got := sumOf(out, prefixEnd); got != wantSum {
		t.Errorf("prefix changed: got %s want %s", got, wantSum)
	}
}

// TestAtMessages_StringWithBracket asserts brackets inside JSON strings
// are NOT counted as array boundaries.
func TestAtMessages_StringWithBracket(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":"check ] this"}]}`)
	payload := []byte(`{"role":"assistant","content":"ok"}`)
	out, err := AtMessages(in, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out) {
		t.Fatalf("not valid: %s", out)
	}
	// The injected payload should appear AFTER the user's message, not
	// inside it.
	if !bytes.Contains(out, []byte(`"check ] this"`)) {
		t.Errorf("user content corrupted: %s", out)
	}
	if !bytes.Contains(out, payload) {
		t.Errorf("payload missing: %s", out)
	}
}

func TestAtMessages_EscapedQuoteInString(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":"with \"quoted\" ]"}]}`)
	payload := []byte(`{"role":"assistant","content":"ok"}`)
	out, err := AtMessages(in, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out) {
		t.Fatalf("not valid: %s", out)
	}
}

func TestAtMessages_MissingKey(t *testing.T) {
	_, err := AtMessages([]byte(`{"foo":"bar"}`), []byte(`{}`))
	if err == nil {
		t.Error("expected error for missing 'messages' key")
	}
}

func TestBeforeCacheBreakpoint_Inserts(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"a","cache_control":{"type":"ephemeral"}}]}]}`)
	payload := []byte(`{"injected":true}`)
	out, err := BeforeCacheBreakpoint(in, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out) {
		t.Fatalf("not valid: %s", out)
	}
	if !bytes.Contains(out, payload) {
		t.Errorf("payload missing: %s", out)
	}
}

func TestBeforeCacheBreakpoint_NoMarker(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":"plain"}]}`)
	_, err := BeforeCacheBreakpoint(in, []byte(`{"x":1}`))
	if !errors.Is(err, ErrNoCacheControl) {
		t.Errorf("err=%v, want ErrNoCacheControl", err)
	}
}

// TestBeforeCacheBreakpoint_PreservesPrefix asserts the bytes before
// the cache_control's containing object are identical across two
// successive injections — the cache-prefix invariant under repeated
// memory injection (brief 002 success criterion 4).
func TestBeforeCacheBreakpoint_PreservesPrefix(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"prelude","cache_control":{"type":"ephemeral"}}]}]}`)

	once, err := BeforeCacheBreakpoint(in, []byte(`{"k":1}`))
	if err != nil {
		t.Fatal(err)
	}
	twice, err := BeforeCacheBreakpoint(in, []byte(`{"k":2}`))
	if err != nil {
		t.Fatal(err)
	}
	// The bytes before the first injected segment should be identical
	// across both runs (and identical to the original prefix).
	prefixEnd := bytes.Index(once, []byte(`{"k":1}`))
	if prefixEnd < 0 {
		t.Fatalf("missing first injection: %s", once)
	}
	twiceEnd := bytes.Index(twice, []byte(`{"k":2}`))
	if twiceEnd != prefixEnd {
		t.Errorf("injection point shifted: once=%d twice=%d", prefixEnd, twiceEnd)
	}
	if !bytes.Equal(once[:prefixEnd], twice[:twiceEnd]) {
		t.Errorf("prefix bytes differ between runs")
	}
	if !bytes.Equal(once[:prefixEnd], in[:prefixEnd]) {
		t.Errorf("prefix differs from original")
	}
}
