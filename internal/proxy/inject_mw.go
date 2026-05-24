package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dreamware-nz/kete/internal/inject"
	"github.com/dreamware-nz/kete/internal/store"
)

// injectMemory splices up to 3 prior tasks for project into rawBody.
// Ranking is "newest first" — real ranking lands in plan 010.
//
// If the body has a cache_control marker, we splice before it (so the
// cache prefix stays intact); otherwise we append to the messages
// array. If there are no prior tasks, the body is returned unchanged.
func injectMemory(ctx context.Context, db *store.DB, project string, rawBody []byte) ([]byte, error) {
	if db == nil || project == "" {
		return rawBody, nil
	}
	tasks, err := db.ListTasks(ctx, project)
	if err != nil {
		return rawBody, fmt.Errorf("inject: list tasks: %w", err)
	}
	if len(tasks) == 0 {
		return rawBody, nil
	}
	if len(tasks) > 3 {
		tasks = tasks[:3]
	}
	payload, err := buildMemoryPayload(tasks)
	if err != nil {
		return rawBody, err
	}
	if out, err := inject.BeforeCacheBreakpoint(rawBody, payload); err == nil {
		return out, nil
	}
	return inject.AtMessages(rawBody, payload)
}

// buildMemoryPayload renders prior tasks as one user message
// containing kete-flavoured XML so the model can recognise the
// segment without us needing to parse it back out. Uses
// inject.Preview so the wire shape (and the 8-char ids) match what
// MCP's kete_expand resolves — see plan 010 phase 6.
func buildMemoryPayload(tasks []*store.Task) ([]byte, error) {
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString(inject.Preview(t))
	}
	msg := map[string]any{
		"role":    "user",
		"content": b.String(),
	}
	return json.Marshal(msg)
}

// injectCorrectionPayload splices a kete-flavoured correction message
// into the request body. Same scheme as memory injection: prefer the
// cache-breakpoint splice; fall back to messages-array tail.
func injectCorrectionPayload(rawBody []byte, correction string) ([]byte, error) {
	msg := map[string]any{
		"role":    "user",
		"content": "<kete:correction>" + escapeXML(correction) + "</kete:correction>",
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return rawBody, err
	}
	if out, err := inject.BeforeCacheBreakpoint(rawBody, payload); err == nil {
		return out, nil
	}
	return inject.AtMessages(rawBody, payload)
}

func escapeXML(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch r {
		case '<':
			out = append(out, []byte("&lt;")...)
		case '>':
			out = append(out, []byte("&gt;")...)
		case '&':
			out = append(out, []byte("&amp;")...)
		default:
			out = append(out, []byte(string(r))...)
		}
	}
	return string(out)
}
