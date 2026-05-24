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
// payload as an SSE event ("event: <type>\ndata: <json>\n\n"). The
// Bedrock convention puts the Anthropic-shaped event JSON in the
// frame's payload; we forward it as-is.
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
		eventType := "message"
		for _, h := range msg.Headers {
			if h.Name == ":event-type" {
				if v, ok := h.Value.Get().(string); ok {
					eventType = v
				}
			}
		}
		// Bedrock encodes the Anthropic event JSON inside payload as
		// {"bytes":"<base64>"}; the Anthropic SSE shape clients want is
		// `event: <type>\ndata: <decoded-json>\n\n`.
		var wrap struct {
			Bytes []byte `json:"bytes"`
		}
		payload := msg.Payload
		if json.Unmarshal(msg.Payload, &wrap) == nil && len(wrap.Bytes) > 0 {
			payload = wrap.Bytes
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}
