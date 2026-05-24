// Package bedrock is the AWS Bedrock upstream adapter.
//
// Differs from anthropic-direct on three axes (ADR 0014):
//  1. SigV4 per request via aws-sdk-go-v2.
//  2. Body re-shape: drop "model", set "anthropic_version" body
//     field, point URL at /model/{id}/invoke[-with-response-stream].
//  3. Event-stream → SSE demux on the response.
package bedrock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
)

const (
	bedrockHostFmt = "bedrock-runtime.%s.amazonaws.com"
	bedrockBodyVer = "bedrock-2023-05-31"
	signingService = "bedrock"
)

// Adapter implements adapter.Wire against AWS Bedrock.
type Adapter struct {
	Region     string
	Creds      aws.CredentialsProvider
	Signer     *v4.Signer
	HTTPClient *http.Client
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
		Region:     region,
		Creds:      cfg.Credentials,
		Signer:     v4.NewSigner(),
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

// Name reports the upstream id.
func (a *Adapter) Name() string { return "bedrock" }

// Forward translates the Anthropic-shaped body into a Bedrock request,
// signs with SigV4, sends, and demuxes the event-stream response into
// SSE for the client.
func (a *Adapter) Forward(ctx context.Context, rawBody []byte, headers http.Header, w http.ResponseWriter) error {
	stream, modelID, body, err := translateRequest(rawBody)
	if err != nil {
		return err
	}
	url := buildURL(a.Region, modelID, stream)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("bedrock: build req: %w", err)
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("content-type", "application/json")
	if stream {
		req.Header.Set("accept", "application/vnd.amazon.eventstream")
	}

	creds, err := a.Creds.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("bedrock: creds: %w", err)
	}
	bodyHash := sha256.Sum256(body)
	if err := a.Signer.SignHTTP(ctx, creds, req,
		hex.EncodeToString(bodyHash[:]), signingService, a.Region, time.Now()); err != nil {
		return fmt.Errorf("bedrock: sign: %w", err)
	}

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("bedrock: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(translateError(resp.StatusCode, body))
		return nil
	}

	if !stream {
		// Non-streaming: pass the JSON body through.
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, err := io.Copy(w, resp.Body)
		return err
	}

	// Streaming: demux event-stream → SSE.
	w.Header().Set("content-type", "text/event-stream")
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	return demuxEventStream(resp.Body, w, flusher)
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

// buildURL points at the regional Bedrock endpoint with the right
// invoke path for stream vs non-stream.
func buildURL(region, modelID string, stream bool) string {
	host := fmt.Sprintf(bedrockHostFmt, region)
	path := "invoke"
	if stream {
		path = "invoke-with-response-stream"
	}
	return fmt.Sprintf("https://%s/model/%s/%s",
		host, strings.ReplaceAll(modelID, "/", "%2F"), path)
}
