package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Upstream is one of the three vendor identifiers from ADR 0015.
type Upstream string

const (
	UpstreamAnthropic Upstream = "anthropic"
	UpstreamCCProxy   Upstream = "cc-proxy"
	UpstreamBedrock   Upstream = "bedrock"
)

// upstreamHeader is the per-request override; consumed and stripped
// before the request hits the adapter.
const upstreamHeader = "X-Kete-Upstream"

// SelectUpstream applies the ADR 0015 precedence to choose the
// upstream for one request: header > model-id pattern > env var.
//
// It also strips x-kete-upstream from headers so it never reaches the
// upstream — that's part of the contract for "consumed and stripped".
//
// rawBody is read read-only to look at the model id; this is a parse,
// never a re-marshal, so the cache prefix is preserved.
func SelectUpstream(headers http.Header, rawBody []byte) (Upstream, error) {
	if v := headers.Get(upstreamHeader); v != "" {
		headers.Del(upstreamHeader)
		switch v {
		case string(UpstreamAnthropic), string(UpstreamCCProxy), string(UpstreamBedrock):
			return Upstream(v), nil
		default:
			return "", fmt.Errorf("invalid %s=%q (want anthropic|cc-proxy|bedrock)", upstreamHeader, v)
		}
	}

	if model := peekModelID(rawBody); model != "" {
		if up := upstreamFromModelID(model); up != "" {
			return up, nil
		}
	}

	if v := os.Getenv("KETE_UPSTREAM"); v != "" {
		switch v {
		case string(UpstreamAnthropic), string(UpstreamCCProxy), string(UpstreamBedrock):
			return Upstream(v), nil
		default:
			return "", fmt.Errorf("invalid KETE_UPSTREAM=%q", v)
		}
	}

	return UpstreamAnthropic, nil
}

// peekModelID does a minimal scan to surface the "model" field's
// string value. We deliberately do not unmarshal into a typed view —
// json.RawMessage + a one-shot decode keeps the rest of the body
// untouched, and we never re-emit anything.
func peekModelID(rawBody []byte) string {
	if len(rawBody) == 0 {
		return ""
	}
	var probe struct {
		Model string `json:"model"`
	}
	// Failures are fine; if the body isn't JSON we just fall through.
	_ = json.Unmarshal(rawBody, &probe)
	return probe.Model
}

// upstreamFromModelID maps a model string to an upstream when the id
// is unambiguous. cc-proxy uses the same model ids as Anthropic-direct
// (per ADR 0015), so it CANNOT be auto-detected.
func upstreamFromModelID(model string) Upstream {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "arn:aws:bedrock:"),
		strings.HasPrefix(m, "us.anthropic."),
		strings.HasPrefix(m, "anthropic.claude"):
		return UpstreamBedrock
	case strings.HasPrefix(m, "claude-"):
		// Ambiguous between anthropic-direct and cc-proxy; let the env
		// or default decide.
		return ""
	}
	return ""
}
