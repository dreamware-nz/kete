package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUsageTap_SSE_AccumulatesAcrossDeltas(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("content-type", "text/event-stream")

	var lastIn, lastOut atomic.Int64
	tap := newUsageTap(rr, func(in, out int) {
		lastIn.Store(int64(in))
		lastOut.Store(int64(out))
	})
	tap.WriteHeader(200)

	frames := []string{
		"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"x\",\"usage\":{\"input_tokens\":1500,\"output_tokens\":1}}}\n\n",
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"a\"}}\n\n",
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":42}}\n\n",
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
	}
	for _, f := range frames {
		if _, err := tap.Write([]byte(f)); err != nil {
			t.Fatal(err)
		}
	}
	tap.Done()

	if got := lastIn.Load(); got != 1500 {
		t.Errorf("input_tokens=%d, want 1500", got)
	}
	if got := lastOut.Load(); got != 42 {
		t.Errorf("output_tokens=%d, want 42 (cumulative from delta)", got)
	}
	// Body passed through unchanged.
	full := strings.Join(frames, "")
	if rr.Body.String() != full {
		t.Errorf("body modified by tap")
	}
}

func TestUsageTap_NonStreamingJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("content-type", "application/json")
	var sawIn, sawOut atomic.Int64
	tap := newUsageTap(rr, func(in, out int) {
		sawIn.Store(int64(in))
		sawOut.Store(int64(out))
	})
	tap.WriteHeader(200)

	body := `{"id":"r","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":7,"output_tokens":3}}`
	if _, err := tap.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	tap.Done()

	if sawIn.Load() != 7 || sawOut.Load() != 3 {
		t.Errorf("got in=%d out=%d, want 7/3", sawIn.Load(), sawOut.Load())
	}
}

func TestUsageTap_FrameSplitAcrossWrites(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("content-type", "text/event-stream")
	var sawOut atomic.Int64
	tap := newUsageTap(rr, func(in, out int) { sawOut.Store(int64(out)) })
	tap.WriteHeader(200)

	frame := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"output_tokens\":2}}}\n\n"
	// Split mid-frame across two Write() calls; the parser should
	// still see one complete frame after the second write.
	mid := len(frame) / 2
	tap.Write([]byte(frame[:mid]))
	if sawOut.Load() != 0 {
		t.Errorf("partial frame leaked: %d", sawOut.Load())
	}
	tap.Write([]byte(frame[mid:]))
	if sawOut.Load() != 2 {
		t.Errorf("got %d, want 2", sawOut.Load())
	}
}

func TestUsageTap_NilCallbackIsPassthrough(t *testing.T) {
	rr := httptest.NewRecorder()
	tap := newUsageTap(rr, nil)
	tap.WriteHeader(200)
	tap.Write([]byte("anything"))
	tap.Done() // must not panic
	if rr.Body.String() != "anything" {
		t.Error("body changed")
	}
}

// flusherWriter implements both http.ResponseWriter and http.Flusher
// so we can assert tap.Flush propagates.
type flusherWriter struct {
	*httptest.ResponseRecorder
	flushes int
}

func (f *flusherWriter) Flush() { f.flushes++ }

func TestUsageTap_FlushPropagates(t *testing.T) {
	fw := &flusherWriter{ResponseRecorder: httptest.NewRecorder()}
	tap := newUsageTap(fw, nil)
	tap.WriteHeader(200)
	tap.Write([]byte("x"))
	tap.Flush()
	tap.Flush()
	if fw.flushes != 2 {
		t.Errorf("flushes=%d, want 2", fw.flushes)
	}
}

// Ensure usageTap satisfies http.ResponseWriter.
var _ http.ResponseWriter = (*usageTap)(nil)
