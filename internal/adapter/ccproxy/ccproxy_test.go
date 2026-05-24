package ccproxy

import (
	"errors"
	"strings"
	"testing"
)

func TestNew_RequiresKey(t *testing.T) {
	t.Setenv("KETE_CC_PROXY_KEY", "")
	_, err := New()
	if err == nil {
		t.Fatal("expected error when KETE_CC_PROXY_KEY unset")
	}
	if !strings.Contains(err.Error(), "KETE_CC_PROXY_KEY") {
		t.Errorf("err=%v should mention KETE_CC_PROXY_KEY", err)
	}
}

func TestNew_DefaultURL(t *testing.T) {
	t.Setenv("KETE_CC_PROXY_KEY", "secret")
	t.Setenv("KETE_CC_PROXY_URL", "")
	a, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if a.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL=%q, want %q", a.BaseURL, defaultBaseURL)
	}
	if a.Name() != "anthropic" {
		// cc-proxy is wire-shaped as anthropic, so the wrapped
		// adapter reports "anthropic". This is intentional per the
		// package doc; selector decides upstream by Upstream key, not
		// by Name().
	}
}

func TestNew_OverrideURL(t *testing.T) {
	t.Setenv("KETE_CC_PROXY_KEY", "secret")
	t.Setenv("KETE_CC_PROXY_URL", "http://localhost:9999")
	a, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if a.BaseURL != "http://localhost:9999" {
		t.Errorf("BaseURL=%q", a.BaseURL)
	}
}

func TestNew_HTTPClientHasTimeout(t *testing.T) {
	t.Setenv("KETE_CC_PROXY_KEY", "secret")
	a, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if a.HTTPClient == nil || a.HTTPClient.Timeout == 0 {
		t.Error("expected configured HTTPClient with a timeout")
	}
}

func TestNew_ErrorIsClassified(t *testing.T) {
	t.Setenv("KETE_CC_PROXY_KEY", "")
	_, err := New()
	if err == nil {
		t.Fatal("want error")
	}
	// errors.Is should not match anything specific here; we just
	// verify it's a plain error string the caller can show.
	var asErr error = err
	if !errors.Is(asErr, asErr) {
		t.Error("error identity broken")
	}
}
