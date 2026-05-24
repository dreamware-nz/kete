package bedrock

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestTranslateRequest(t *testing.T) {
	in := []byte(`{"model":"anthropic.claude-3-5-sonnet-20240620-v1:0","max_tokens":1000,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	stream, modelID, body, err := translateRequest(in)
	if err != nil {
		t.Fatal(err)
	}
	if !stream {
		t.Error("stream=false, want true")
	}
	if modelID != "anthropic.claude-3-5-sonnet-20240620-v1:0" {
		t.Errorf("modelID=%q", modelID)
	}
	var probe map[string]any
	if err := json.Unmarshal(body, &probe); err != nil {
		t.Fatal(err)
	}
	if _, hasModel := probe["model"]; hasModel {
		t.Error("model not stripped from body")
	}
	// Bedrock's /invoke-with-response-stream rejects 'stream' in the
	// body — verified live against bedrock-runtime. Strip it.
	if _, hasStream := probe["stream"]; hasStream {
		t.Error("stream not stripped from body (Bedrock rejects it)")
	}
	if probe["anthropic_version"] != bedrockBodyVer {
		t.Errorf("anthropic_version=%v", probe["anthropic_version"])
	}
}

func TestBuildURL(t *testing.T) {
	u := buildURL("us-east-1", "anthropic.claude-3-5-sonnet-20240620-v1:0", true)
	if !strings.Contains(u, "bedrock-runtime.us-east-1.amazonaws.com") {
		t.Errorf("missing region host: %s", u)
	}
	if !strings.Contains(u, "invoke-with-response-stream") {
		t.Errorf("not streaming path: %s", u)
	}
	u = buildURL("eu-west-1", "model-id", false)
	if !strings.HasSuffix(u, "/invoke") || strings.Contains(u, "response-stream") {
		t.Errorf("non-stream URL: %s", u)
	}
}

func TestTranslateError_WrapsBedrockShape(t *testing.T) {
	in := []byte(`{"__type":"ValidationException","message":"bad request"}`)
	out := translateError(400, in)
	if !bytes.Contains(out, []byte("ValidationException")) {
		t.Errorf("missing type: %s", out)
	}
	if !bytes.Contains(out, []byte(`"type":"error"`)) {
		t.Errorf("not error-shaped: %s", out)
	}
}

func TestTranslateError_FallbackOnNonJSON(t *testing.T) {
	out := translateError(503, []byte("upstream busy"))
	if !bytes.Contains(out, []byte("upstream busy")) {
		t.Errorf("message lost: %s", out)
	}
}

// TestAnthropicEventType: event name comes from the *inner* JSON
// payload's `type`, not Bedrock's outer "chunk" envelope. Verified
// live: Anthropic clients dispatch on the SSE event line, so we must
// emit `event: message_start` etc., not `event: chunk`.
func TestAnthropicEventType(t *testing.T) {
	cases := []struct {
		payload string
		want    string
	}{
		{`{"type":"message_start","message":{"id":"x"}}`, "message_start"},
		{`{"type":"content_block_delta","index":0,"delta":{"text":"hi"}}`, "content_block_delta"},
		{`{"type":"message_delta","usage":{"output_tokens":5}}`, "message_delta"},
		{`{"type":"message_stop"}`, "message_stop"},
		{`not json`, ""},
	}
	for _, c := range cases {
		if got := anthropicEventType([]byte(c.payload)); got != c.want {
			t.Errorf("payload=%q got=%q want=%q", c.payload, got, c.want)
		}
	}
}
