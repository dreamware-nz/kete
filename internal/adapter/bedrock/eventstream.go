package bedrock

import (
	"encoding/json"
	"fmt"
)

// translateError wraps a Bedrock error response in Anthropic-shaped
// JSON so the client doesn't need to know it's Bedrock.
func translateError(status int, body []byte) []byte {
	var probe struct {
		Type    string `json:"__type"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &probe)
	if probe.Type == "" {
		probe.Type = fmt.Sprintf("bedrock_%d", status)
	}
	if probe.Message == "" {
		probe.Message = string(body)
	}
	out := map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    probe.Type,
			"message": probe.Message,
		},
	}
	b, _ := json.Marshal(out)
	return b
}

// anthropicEventType pulls the inner `type` field from a JSON payload
// (e.g. `{"type":"message_start", ...}` → "message_start"). Used by
// the streaming forward path to label each SSE frame with the
// Anthropic event name rather than Bedrock's "chunk" envelope.
func anthropicEventType(payload []byte) string {
	var probe struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(payload, &probe)
	return probe.Type
}
