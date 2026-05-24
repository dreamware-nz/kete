package anthropic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestForward_ByteExact_RequestBody(t *testing.T) {
	wireIn := []byte(`{"model":"claude","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	want := sha256.Sum256(wireIn)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ := io.ReadAll(r.Body)
		if h := sha256.Sum256(got); !bytes.Equal(h[:], want[:]) {
			t.Errorf("upstream body sha=%s want=%s",
				hex.EncodeToString(h[:8]), hex.EncodeToString(want[:8]))
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	a := &Adapter{BaseURL: upstream.URL, HTTPClient: http.DefaultClient}
	rr := httptest.NewRecorder()
	if err := a.Forward(context.Background(), wireIn,
		http.Header{"Content-Type": []string{"application/json"}}, rr); err != nil {
		t.Fatal(err)
	}
	if rr.Code != 200 {
		t.Errorf("status=%d, want 200", rr.Code)
	}
}

func TestForward_SSE_Streams(t *testing.T) {
	const chunks = 4
	chunkInterval := 50 * time.Millisecond

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
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

	a := &Adapter{BaseURL: upstream.URL, HTTPClient: http.DefaultClient}

	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = a.Forward(r.Context(), body, r.Header, w)
	}))
	defer front.Close()

	resp, err := http.Get(front.URL + "/v1/messages")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var arrivals []time.Time
	buf := make([]byte, 1)
	var line strings.Builder
	deadline := time.Now().Add(2 * time.Second)
	for len(arrivals) < chunks && time.Now().Before(deadline) {
		n, err := resp.Body.Read(buf)
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
	}
	if len(arrivals) != chunks {
		t.Fatalf("got %d, want %d", len(arrivals), chunks)
	}
	for i := 1; i < len(arrivals); i++ {
		gap := arrivals[i].Sub(arrivals[i-1])
		if gap < chunkInterval-15*time.Millisecond {
			t.Errorf("chunk %d gap %s, want >= %s", i, gap, chunkInterval)
		}
	}
}
