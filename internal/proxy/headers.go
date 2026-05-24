package proxy

import (
	"net/http"
	"strings"
)

// forwardedHeaders is the brief 002 whitelist. Lowercase, used for
// case-insensitive lookups since http.Header normalises keys.
var forwardedHeaders = map[string]struct{}{
	"x-api-key":         {},
	"authorization":     {},
	"anthropic-version": {},
	"content-type":      {},
	"anthropic-beta":    {},
}

// secretHeaders are the keys that must never appear in plaintext logs.
var secretHeaders = map[string]struct{}{
	"x-api-key":     {},
	"authorization": {},
}

// SanitiseHeaders returns a new Header containing only the whitelisted
// keys from in, preserving their original casing on the way out via
// http.CanonicalHeaderKey. Anything not on the list is dropped, with
// no exceptions for "kete-internal" headers — those are stripped by
// the upstream selector before this is called.
func SanitiseHeaders(in http.Header) http.Header {
	out := make(http.Header, len(forwardedHeaders))
	for k, vs := range in {
		if _, ok := forwardedHeaders[strings.ToLower(k)]; !ok {
			continue
		}
		out[http.CanonicalHeaderKey(k)] = append([]string(nil), vs...)
	}
	return out
}

// RedactForLog returns a copy of in with secret-bearing values replaced
// by "[REDACTED]". Used by the request logger; the forwarder never
// calls this — it forwards bytes, not the redacted view.
func RedactForLog(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for k, vs := range in {
		if _, isSecret := secretHeaders[strings.ToLower(k)]; isSecret {
			out[http.CanonicalHeaderKey(k)] = []string{"[REDACTED]"}
			continue
		}
		// Belt-and-braces: anything that *looks* AWS-credential-shaped
		// gets redacted too. Brief 002 mentions the AWS pattern.
		if isAWSCredHeader(k) {
			out[http.CanonicalHeaderKey(k)] = []string{"[REDACTED]"}
			continue
		}
		out[http.CanonicalHeaderKey(k)] = append([]string(nil), vs...)
	}
	return out
}

// isAWSCredHeader matches the AWS SigV4 header family. Bedrock requests
// will carry these (plan 012); when we log them they get redacted.
func isAWSCredHeader(k string) bool {
	lk := strings.ToLower(k)
	return strings.HasPrefix(lk, "x-amz-security-token") ||
		strings.HasPrefix(lk, "x-amz-date") ||
		strings.HasPrefix(lk, "x-amz-content-sha256") ||
		lk == "authorization" // also caught above; defence in depth
}
