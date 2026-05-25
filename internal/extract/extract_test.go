package extract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// stub builds an httptest server that returns a fixed Anthropic-shaped
// response. Useful for asserting client request shape.
func stub(t *testing.T, status int, content string, capture *atomic.Pointer[Request]) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected req %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}
		if r.Header.Get("x-api-key") == "" {
			t.Error("missing x-api-key header")
		}
		if capture != nil {
			var req Request
			_ = json.NewDecoder(r.Body).Decode(&req)
			capture.Store(&req)
		}
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"id":"x","type":"message","role":"assistant","content":[{"type":"text","text":%q}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":2}}`, content)
	}))
}

func newClientFor(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("KETE_ANTHROPIC_URL", srv.URL)
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSend_HappyPath(t *testing.T) {
	srv := stub(t, 200, `{"goal":"test","decisions":[],"files_touched":[]}`, nil)
	defer srv.Close()
	c := newClientFor(t, srv)

	resp, err := c.Send(context.Background(), Request{
		MaxTokens: 256,
		Messages:  []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Text(); !strings.Contains(got, "test") {
		t.Errorf("got %q", got)
	}
}

func TestSendWithRetry_5xxThen200(t *testing.T) {
	var seen atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := seen.Add(1)
		if n == 1 {
			w.WriteHeader(500)
			return
		}
		fmt.Fprint(w, `{"id":"x","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer srv.Close()
	c := newClientFor(t, srv)
	resp, err := c.SendWithRetry(context.Background(), Request{
		MaxTokens: 256,
		Messages:  []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Text(), "ok") {
		t.Errorf("got %s", resp.Text())
	}
	if seen.Load() != 2 {
		t.Errorf("attempts=%d, want 2", seen.Load())
	}
}

func TestSendWithRetry_400_FailsFast(t *testing.T) {
	var seen atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Add(1)
		http.Error(w, `{"error":{"message":"bad"}}`, 400)
	}))
	defer srv.Close()
	c := newClientFor(t, srv)
	_, err := c.SendWithRetry(context.Background(), Request{
		MaxTokens: 256,
		Messages:  []Message{{Role: "user", Content: "hi"}},
	})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 400 {
		t.Errorf("err=%v, want 400 HTTPError", err)
	}
	if seen.Load() != 1 {
		t.Errorf("attempts=%d, want 1 (fail-fast)", seen.Load())
	}
}

func TestExtractTask_ParsesJSON(t *testing.T) {
	body := `"goal":"refactor auth","decisions":[{"choice":"jwt","rationale":"stateless"}],"files_touched":["a.go"]}`
	srv := stub(t, 200, body, nil)
	defer srv.Close()
	c := newClientFor(t, srv)
	out, err := c.ExtractTask(context.Background(), "user did some stuff")
	if err != nil {
		t.Fatal(err)
	}
	if out.Goal != "refactor auth" {
		t.Errorf("goal=%q", out.Goal)
	}
	if len(out.Decisions) != 1 || out.Decisions[0].Choice != "jwt" {
		t.Errorf("decisions=%+v", out.Decisions)
	}
	if len(out.FilesTouched) != 1 || out.FilesTouched[0] != "a.go" {
		t.Errorf("files=%+v", out.FilesTouched)
	}
}

func TestExtractDecisions_ParsesJSON(t *testing.T) {
	body := `{"decisions":[{"choice":"X","rationale":"because"}]}`
	srv := stub(t, 200, body, nil)
	defer srv.Close()
	c := newClientFor(t, srv)
	out, err := c.ExtractDecisions(context.Background(), "...")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Decisions) != 1 {
		t.Errorf("decisions=%+v", out.Decisions)
	}
}

func TestMaxTokens_RespectedPerCallSite(t *testing.T) {
	var captured atomic.Pointer[Request]
	srv := stub(t, 200, `"goal":"","decisions":[],"files_touched":[]}`, &captured)
	defer srv.Close()
	c := newClientFor(t, srv)
	if _, err := c.ExtractTask(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if r := captured.Load(); r == nil || r.MaxTokens > MaxTokensExtractTask {
		t.Errorf("max_tokens=%d, want <= %d", r.MaxTokens, MaxTokensExtractTask)
	}
	if r := captured.Load(); r == nil || r.MaxTokens != MaxTokensExtractTask {
		t.Errorf("max_tokens=%d, want exactly %d", r.MaxTokens, MaxTokensExtractTask)
	}
}

func TestPrompts_AllPresent(t *testing.T) {
	for k, v := range Prompts {
		if strings.TrimSpace(v) == "" {
			t.Errorf("prompt %q is empty", k)
		}
	}
	want := []string{"extract_task", "extract_decisions", "drift_score", "drift_correct", "compact_summary"}
	for _, k := range want {
		if _, ok := Prompts[k]; !ok {
			t.Errorf("missing prompt %q", k)
		}
	}
}
