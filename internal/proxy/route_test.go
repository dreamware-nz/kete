package proxy

import (
	"net/http"
	"testing"
)

func TestSelectUpstream_HeaderWins(t *testing.T) {
	t.Setenv("KETE_UPSTREAM", "bedrock")
	hdr := http.Header{upstreamHeader: []string{"cc-proxy"}}
	got, err := SelectUpstream(hdr, []byte(`{"model":"anthropic.claude-3-5-sonnet"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != UpstreamCCProxy {
		t.Errorf("got %q, want cc-proxy", got)
	}
	if hdr.Get(upstreamHeader) != "" {
		t.Errorf("upstream header leaked: %q", hdr.Get(upstreamHeader))
	}
}

func TestSelectUpstream_ModelIDWins_OverEnv(t *testing.T) {
	t.Setenv("KETE_UPSTREAM", "anthropic")
	got, err := SelectUpstream(http.Header{},
		[]byte(`{"model":"arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != UpstreamBedrock {
		t.Errorf("got %q, want bedrock", got)
	}
}

func TestSelectUpstream_EnvFallback(t *testing.T) {
	t.Setenv("KETE_UPSTREAM", "cc-proxy")
	got, err := SelectUpstream(http.Header{}, []byte(`{"model":"claude-sonnet-4-5"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != UpstreamCCProxy {
		t.Errorf("got %q, want cc-proxy", got)
	}
}

func TestSelectUpstream_DefaultAnthropic(t *testing.T) {
	t.Setenv("KETE_UPSTREAM", "")
	got, err := SelectUpstream(http.Header{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != UpstreamAnthropic {
		t.Errorf("got %q, want anthropic", got)
	}
}

func TestSelectUpstream_RejectsBadValues(t *testing.T) {
	hdr := http.Header{upstreamHeader: []string{"oopsie"}}
	if _, err := SelectUpstream(hdr, nil); err == nil {
		t.Error("expected error on bad header value")
	}
	t.Setenv("KETE_UPSTREAM", "oopsie")
	if _, err := SelectUpstream(http.Header{}, nil); err == nil {
		t.Error("expected error on bad env value")
	}
}
