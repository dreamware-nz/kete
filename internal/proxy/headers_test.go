package proxy

import (
	"net/http"
	"testing"
)

func TestSanitiseHeaders_KeepsWhitelist_DropsRest(t *testing.T) {
	in := http.Header{
		"X-Api-Key":         []string{"sk-test"},
		"Authorization":     []string{"Bearer abc"},
		"Anthropic-Version": []string{"2023-06-01"},
		"Content-Type":      []string{"application/json"},
		"Anthropic-Beta":    []string{"prompt-caching-2024-07-31"},
		"User-Agent":        []string{"crush/0.1"},
		"Cookie":            []string{"forbidden"},
		"X-Kete-Upstream":   []string{"bedrock"},
	}
	out := SanitiseHeaders(in)
	for k := range forwardedHeaders {
		canon := http.CanonicalHeaderKey(k)
		if out.Get(canon) == "" {
			t.Errorf("missing whitelisted %s", canon)
		}
	}
	for _, drop := range []string{"User-Agent", "Cookie", "X-Kete-Upstream"} {
		if out.Get(drop) != "" {
			t.Errorf("%s leaked through", drop)
		}
	}
}

func TestRedactForLog_HidesSecrets(t *testing.T) {
	in := http.Header{
		"X-Api-Key":            []string{"sk-secret"},
		"Authorization":        []string{"Bearer abc"},
		"X-Amz-Security-Token": []string{"AWS-secret"},
		"X-Amz-Date":           []string{"20260524T120000Z"},
		"Content-Type":         []string{"application/json"},
	}
	out := RedactForLog(in)
	for _, k := range []string{"X-Api-Key", "Authorization", "X-Amz-Security-Token", "X-Amz-Date"} {
		if got := out.Get(k); got != "[REDACTED]" {
			t.Errorf("%s = %q, want [REDACTED]", k, got)
		}
	}
	if got := out.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type leaked-through = %q", got)
	}
}
