package capture

import (
	"strings"
	"testing"
)

func TestClipTrace_UnderCap(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	got := ClipTrace(body)
	if got != string(body) {
		t.Fatalf("under-cap body mutated:\n got=%q\nwant=%q", got, body)
	}
	if strings.Contains(got, TruncatedMarker) {
		t.Fatalf("under-cap body should not carry the truncation marker")
	}
}

func TestClipTrace_OverCap_KeepsTail(t *testing.T) {
	head := strings.Repeat("HEAD", TraceCap)
	tail := `..."the-only-thing-that-matters"}}`
	body := []byte(head + tail)
	got := ClipTrace(body)
	if len(got) > TraceCap {
		t.Fatalf("clipped trace len=%d, want <= %d", len(got), TraceCap)
	}
	if !strings.HasPrefix(got, TruncatedMarker) {
		t.Fatalf("clipped trace missing marker prefix; got prefix=%q", got[:64])
	}
	if !strings.HasSuffix(got, tail) {
		t.Fatalf("clipped trace dropped the tail; got suffix=%q", got[len(got)-len(tail):])
	}
}

func TestClipTrace_AtCap_Unchanged(t *testing.T) {
	body := []byte(strings.Repeat("a", TraceCap))
	got := ClipTrace(body)
	if len(got) != TraceCap {
		t.Fatalf("at-cap len=%d, want %d", len(got), TraceCap)
	}
	if strings.Contains(got, TruncatedMarker) {
		t.Fatalf("at-cap body should not carry the truncation marker")
	}
}
