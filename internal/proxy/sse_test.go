package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestForward_SSE_FlushesPerChunk asserts the per-chunk arrival times
// at the client mirror the upstream pacing. Each chunk that left the
// upstream `interval` apart must arrive at the client `interval`
// apart, within tolerance.
func TestForward_SSE_FlushesPerChunk(t *testing.T) {
	const chunks = 4
	chunkInterval := 50 * time.Millisecond

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		for i := 0; i < chunks; i++ {
			fmt.Fprintf(w, "data: chunk-%d\n\n", i)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(chunkInterval)
		}
	}))
	defer upstream.Close()

	// Front-side proxy: a tiny test server that calls forward(...) for
	// every request, echoing the upstream's SSE through.
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = forward(r.Context(), http.DefaultClient, http.MethodGet,
			upstream.URL+"/v1/messages", body, r.Header, w)
	}))
	defer front.Close()

	resp, err := http.Get(front.URL + "/v1/messages")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	arrivals := readSSEChunkArrivals(t, resp.Body, chunks)
	if len(arrivals) != chunks {
		t.Fatalf("got %d arrivals, want %d", len(arrivals), chunks)
	}
	for i := 1; i < len(arrivals); i++ {
		gap := arrivals[i].Sub(arrivals[i-1])
		if gap < chunkInterval-15*time.Millisecond {
			t.Errorf("chunk %d arrived %s after previous, want >= %s",
				i, gap, chunkInterval)
		}
	}
}

// readSSEChunkArrivals reads `chunks` SSE data: lines and records the
// arrival time of each.
func readSSEChunkArrivals(t *testing.T, r io.Reader, chunks int) []time.Time {
	t.Helper()
	var arrivals []time.Time
	buf := make([]byte, 1)
	var line strings.Builder
	deadline := time.Now().Add(2 * time.Second)
	for len(arrivals) < chunks && time.Now().Before(deadline) {
		n, err := r.Read(buf)
		if n > 0 {
			line.WriteByte(buf[0])
			if buf[0] == '\n' {
				if strings.HasPrefix(line.String(), "data: ") {
					arrivals = append(arrivals, time.Now())
				}
				line.Reset()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return arrivals
}

// silence unused-import grumble in this file.
var _ = context.Background
