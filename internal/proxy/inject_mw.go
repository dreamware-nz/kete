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
// Splices at the end of the messages array via inject.AtMessages.
//
// We previously tried inject.BeforeCacheBreakpoint to land before
// the first cache_control marker (preserving the Anthropic
// prompt-cache prefix), but the walkBackToObjectStart heuristic in
// internal/inject misidentifies the containing message when
// cache_control sits deep inside a content block — it splices INTO a
// content array instead of BETWEEN messages, producing JSON that
// Bedrock reasonably rejects with
// "messages.N.content.M.type: Field required". Live-caught against
// Crush + Bedrock. Until inject.BeforeCacheBreakpoint can be made
// message-aware, AtMessages is the only correct path.
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
	return inject.AtMessages(rawBody, payload)
}

// buildMemoryPayload renders prior tasks as one user message
// containing kete-flavoured XML so the model can recognise the
// segment without us needing to parse it back out. Uses
// inject.Preview so the wire shape (and the 8-char ids) match what
// MCP's kete_expand resolves — see plan 010 phase 6.
//
// Content is emitted as a list of typed blocks rather than a bare
// string. Anthropic-direct accepts both shapes; Bedrock-on-Anthropic
// requires the typed-block form ("messages.N.content.0.type: Field
// required" otherwise). Live-caught against Crush + Bedrock.
func buildMemoryPayload(tasks []*store.Task) ([]byte, error) {
	var b strings.Builder
	for _, t := range tasks {
		b.WriteString(inject.Preview(t))
	}
	msg := map[string]any{
		"role": "user",
		"content": []any{
			map[string]any{"type": "text", "text": b.String()},
		},
	}
	return json.Marshal(msg)
}

// injectCorrectionPayload splices a kete-flavoured correction message
// at the end of the messages array. Same Bedrock-friendly typed-block
// content shape. Note: also avoids inject.BeforeCacheBreakpoint for
// the same body-corruption reason as injectMemory.
func injectCorrectionPayload(rawBody []byte, correction string) ([]byte, error) {
	msg := map[string]any{
		"role": "user",
		"content": []any{
			map[string]any{
				"type": "text",
				"text": "<kete:correction>" + escapeXML(correction) + "</kete:correction>",
			},
		},
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return rawBody, err
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
