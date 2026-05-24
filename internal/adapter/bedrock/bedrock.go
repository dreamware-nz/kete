// Package bedrock is the AWS Bedrock upstream adapter.
//
// Differs from anthropic-direct on three axes (ADR 0014):
//  1. Auth: SigV4. We delegate to the bedrockruntime SDK client
//     rather than hand-rolling SigV4+HTTP — the SDK has middleware
//     that the raw signer/v4 path bypasses (content-type defaulting,
//     chunked-payload signing nuances, etc.). Live-caught: the raw
//     path made Bedrock 400 on bodies the SDK accepts byte-identically.
//  2. Body re-shape: drop "model" and "stream" (model goes in the
//     InvokeModelInput; the stream choice picks Invoke vs
//     InvokeModelWithResponseStream), set "anthropic_version".
//  3. Response framing: the SDK's stream output is Anthropic event
//     JSON wrapped in {"bytes":"..."}; we re-emit as Anthropic-shaped
//     SSE so clients dispatch on the inner type.
package bedrock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	smithy "github.com/aws/smithy-go"
)

const bedrockBodyVer = "bedrock-2023-05-31"

// Adapter implements adapter.Wire against AWS Bedrock.
type Adapter struct {
	Region string
	Client *bedrockruntime.Client
}

// New resolves credentials via the standard AWS chain. Returns an
// error if the region or credentials can't be resolved — callers can
// then 501 cleanly without trying to forward.
func New(ctx context.Context) (*Adapter, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		return nil, errors.New("bedrock: AWS_REGION not set")
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("bedrock: load aws config: %w", err)
	}
	return &Adapter{
		Region: region,
		Client: bedrockruntime.NewFromConfig(cfg),
	}, nil
}

// Name reports the upstream id.
func (a *Adapter) Name() string { return "bedrock" }

// Forward translates the Anthropic-shaped body into a Bedrock
// invocation. The SDK does the SigV4, content-type, and request
// shaping for us — kete just translates the body shape (ADR 0014's
// deliberate exception to ADR 0006).
func (a *Adapter) Forward(ctx context.Context, rawBody []byte, _ http.Header, w http.ResponseWriter) error {
	stream, modelID, body, err := translateRequest(rawBody)
	if err != nil {
		return err
	}

	if stream {
		return a.forwardStream(ctx, modelID, body, w)
	}
	return a.forwardUnary(ctx, modelID, body, w)
}

func (a *Adapter) forwardUnary(ctx context.Context, modelID string, body []byte, w http.ResponseWriter) error {
	out, err := a.Client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return writeBedrockError(w, err)
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, werr := w.Write(out.Body)
	return werr
}

func (a *Adapter) forwardStream(ctx context.Context, modelID string, body []byte, w http.ResponseWriter) error {
	out, err := a.Client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return writeBedrockError(w, err)
	}
	defer out.GetStream().Close()

	w.Header().Set("content-type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)

	for ev := range out.GetStream().Events() {
		chunk, ok := ev.(*types.ResponseStreamMemberChunk)
		if !ok {
			continue
		}
		// chunk.Value.Bytes is the Anthropic event JSON. Pull the
		// inner `type` to use as the SSE event name (so clients
		// dispatch identically to Anthropic-direct).
		eventType := anthropicEventType(chunk.Value.Bytes)
		if eventType == "" {
			eventType = "message"
		}
		if _, werr := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, chunk.Value.Bytes); werr != nil {
			return werr
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	if streamErr := out.GetStream().Err(); streamErr != nil {
		// Stream errors land mid-flight after headers are flushed;
		// surface as a synthetic SSE error event so the client sees
		// something instead of a silent close.
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", translateError(0, []byte(streamErr.Error())))
		if flusher != nil {
			flusher.Flush()
		}
	}
	return nil
}

// translateRequest reads the Anthropic-shaped JSON, returns the model
// id (extracted), the stream flag, and a re-marshalled body suitable
// for Bedrock. ADR 0014's deliberate exception to ADR 0006.
func translateRequest(rawBody []byte) (stream bool, modelID string, body []byte, err error) {
	var probe map[string]any
	if err := json.Unmarshal(rawBody, &probe); err != nil {
		return false, "", nil, fmt.Errorf("bedrock: parse body: %w", err)
	}
	m, _ := probe["model"].(string)
	modelID = m
	if s, ok := probe["stream"].(bool); ok {
		stream = s
	}
	delete(probe, "model")
	// Bedrock's /invoke-with-response-stream endpoint encodes the
	// streaming choice in the URL path; the field is rejected in the
	// body. /invoke is non-streaming by definition. Drop either way.
	delete(probe, "stream")
	if _, ok := probe["anthropic_version"]; !ok {
		probe["anthropic_version"] = bedrockBodyVer
	}
	body, err = json.Marshal(probe)
	return stream, modelID, body, err
}

// writeBedrockError renders an SDK error as an Anthropic-shaped error
// JSON response and returns nil (the request succeeded, the upstream
// said no). Status comes from the smithy APIError when available.
func writeBedrockError(w http.ResponseWriter, sdkErr error) error {
	status := http.StatusBadGateway
	body := []byte(sdkErr.Error())

	var apiErr smithy.APIError
	if errors.As(sdkErr, &apiErr) {
		switch apiErr.ErrorFault() {
		case smithy.FaultClient:
			status = http.StatusBadRequest
		case smithy.FaultServer:
			status = http.StatusBadGateway
		}
		// Best-effort: pull any nested response body.
		if rerr, ok := sdkErr.(interface{ HTTPStatusCode() int }); ok {
			status = rerr.HTTPStatusCode()
		}
		// API error has Code() + Message().
		body, _ = json.Marshal(map[string]any{
			"__type":  apiErr.ErrorCode(),
			"message": apiErr.ErrorMessage(),
		})
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(translateError(status, body))
	return nil
}
