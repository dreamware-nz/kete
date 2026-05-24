package bedrock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
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

// demuxEventStream reads AWS event-stream frames and re-emits each
// payload as an SSE event matching what Anthropic-direct emits:
// `event: <type>\ndata: <json>\n\n`, where <type> is the Anthropic
// event name (`message_start`, `content_block_delta`, etc.) — NOT
// the Bedrock outer envelope name (`chunk`).
//
// Anthropic clients dispatch on the SSE event line, so we extract
// the inner `type` from the JSON payload and use it.
func demuxEventStream(r io.Reader, w http.ResponseWriter, flusher http.Flusher) error {
	dec := eventstream.NewDecoder()
	buf := make([]byte, 0, 64*1024)
	for {
		msg, err := dec.Decode(r, buf)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("bedrock eventstream: %w", err)
		}
		// Bedrock encodes the Anthropic event JSON inside payload as
		// {"bytes":"<base64>"}.
		var wrap struct {
			Bytes []byte `json:"bytes"`
		}
		payload := msg.Payload
		if json.Unmarshal(msg.Payload, &wrap) == nil && len(wrap.Bytes) > 0 {
			payload = wrap.Bytes
		}
		// Pull the Anthropic event type from the decoded payload's
		// `type` field. Fall back to the Bedrock outer event-type
		// header when the payload is not JSON (shouldn't happen for
		// Anthropic models, but defends against unexpected shapes).
		eventType := anthropicEventType(payload)
		if eventType == "" {
			eventType = bedrockEventType(msg.Headers)
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// anthropicEventType pulls the inner `type` field from a JSON payload
// (e.g. `{"type":"message_start", ...}` → "message_start").
func anthropicEventType(payload []byte) string {
	var probe struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(payload, &probe)
	return probe.Type
}

// bedrockEventType is the fallback — Bedrock's outer envelope event.
func bedrockEventType(headers []eventstream.Header) string {
	for _, h := range headers {
		if h.Name == ":event-type" {
			if v, ok := h.Value.Get().(string); ok {
				return v
			}
		}
	}
	return "message"
}
