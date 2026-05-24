package compact

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dreamware-nz/kete/internal/extract"
)

func newStubClient(t *testing.T, body string) *extract.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"id":"x","type":"message","role":"assistant","content":[{"type":"text","text":%q}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`, body)
	}))
	t.Cleanup(srv.Close)
	return &extract.Client{
		BaseURL: srv.URL, APIKey: "sk-test",
		Model: "test", HTTPClient: http.DefaultClient,
	}
}

func TestCompute_ParsesSummary(t *testing.T) {
	body := `{"summary":{"goal":"login","decisions":[{"choice":"jwt","rationale":"stateless"}],"constraints":["no breaking change"],"current_state":"flow built; tests pending","open_questions":["which auth provider"]}}`
	c := newStubClient(t, body)
	s, err := Compute(context.Background(), c, "long conversation here")
	if err != nil {
		t.Fatal(err)
	}
	if s.Goal != "login" {
		t.Errorf("goal=%q", s.Goal)
	}
	if len(s.Decisions) != 1 || s.Decisions[0].Choice != "jwt" {
		t.Errorf("decisions=%+v", s.Decisions)
	}
	if len(s.Constraints) != 1 || s.Constraints[0] != "no breaking change" {
		t.Errorf("constraints=%+v", s.Constraints)
	}
	if s.CurrentState == "" || len(s.OpenQuestions) != 1 {
		t.Errorf("current_state=%q open=%v", s.CurrentState, s.OpenQuestions)
	}
}

func TestCache_StartCompute_GetWaits(t *testing.T) {
	body := `{"summary":{"goal":"x","decisions":[],"constraints":[],"current_state":"y","open_questions":[]}}`
	c := newStubClient(t, body)
	cache := NewCache()
	cache.StartCompute(context.Background(), c, "session-1", "convo")
	cache.Wait("session-1")
	s, ok := cache.Get("session-1")
	if !ok {
		t.Fatal("expected summary in cache")
	}
	if s.Goal != "x" {
		t.Errorf("goal=%q", s.Goal)
	}
}

func TestCache_StartCompute_Idempotent(t *testing.T) {
	body := `{"summary":{"goal":"x","decisions":[],"constraints":[],"current_state":"y","open_questions":[]}}`
	c := newStubClient(t, body)
	cache := NewCache()
	cache.StartCompute(context.Background(), c, "s", "convo")
	cache.StartCompute(context.Background(), c, "s", "convo") // no-op
	cache.Wait("s")
	if _, ok := cache.Get("s"); !ok {
		t.Fatal("expected summary")
	}
	// Second StartCompute on a session with a stored summary is a
	// no-op too.
	cache.StartCompute(context.Background(), c, "s", "convo")
	if _, ok := cache.Get("s"); !ok {
		t.Fatal("summary disappeared after no-op StartCompute")
	}
}
