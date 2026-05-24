package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// usageTap wraps an http.ResponseWriter, forwards every byte
// unmodified to the inner writer, and side-bands Anthropic usage
// numbers out via onUsage.
//
// Two response shapes are supported:
//   - SSE (`text/event-stream`): parse `event: message_start` for
//     initial input/output tokens, `event: message_delta` for the
//     latest cumulative output tokens.
//   - non-streaming JSON: parse the accumulated body once at Done()
//     and pull `usage`.
//
// onUsage is called every time we observe a new usage number with
// cumulative (input, output) totals. compactHook.Observe is hysteretic
// so duplicate calls are safe.
type usageTap struct {
	inner   http.ResponseWriter
	flusher http.Flusher
	onUsage func(inputTokens, outputTokens int)

	mu           sync.Mutex
	inputTokens  int
	outputTokens int

	sseBuf   []byte
	sseMode  bool
	sseSeen  bool // any frame seen yet?
	finished bool
}

// newUsageTap wraps w. cb may be nil (then the tap is a transparent
// passthrough — useful when compaction isn't configured).
func newUsageTap(w http.ResponseWriter, cb func(int, int)) *usageTap {
	flusher, _ := w.(http.Flusher)
	return &usageTap{inner: w, flusher: flusher, onUsage: cb}
}

func (u *usageTap) Header() http.Header { return u.inner.Header() }

func (u *usageTap) WriteHeader(status int) {
	ct := u.inner.Header().Get("content-type")
	u.sseMode = strings.HasPrefix(ct, "text/event-stream")
	u.inner.WriteHeader(status)
}

func (u *usageTap) Write(p []byte) (int, error) {
	n, err := u.inner.Write(p)
	if u.onUsage != nil && n > 0 {
		u.mu.Lock()
		u.sseBuf = append(u.sseBuf, p[:n]...)
		if u.sseMode {
			u.consumeFramesLocked()
		}
		u.mu.Unlock()
	}
	return n, err
}

// Flush passes through. SSE clients depend on per-chunk flush.
func (u *usageTap) Flush() {
	if u.flusher != nil {
		u.flusher.Flush()
	}
}

// Done finalises the tap. For non-streaming responses the JSON body
// is parsed here. Always idempotent. Caller (handleMessages) defers
// this after Forward.
func (u *usageTap) Done() {
	if u.onUsage == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.finished {
		return
	}
	u.finished = true
	if !u.sseMode {
		u.parseJSONLocked()
	}
	if u.sseMode && !u.sseSeen {
		// Stream closed without any usage frames — nothing to report.
		return
	}
	u.onUsage(u.inputTokens, u.outputTokens)
}

func (u *usageTap) consumeFramesLocked() {
	for {
		idx := bytes.Index(u.sseBuf, []byte("\n\n"))
		if idx < 0 {
			return
		}
		frame := u.sseBuf[:idx]
		u.sseBuf = u.sseBuf[idx+2:]
		u.processFrameLocked(frame)
	}
}

func (u *usageTap) processFrameLocked(frame []byte) {
	var event string
	var data []byte
	for _, line := range bytes.Split(frame, []byte("\n")) {
		switch {
		case bytes.HasPrefix(line, []byte("event: ")):
			event = string(line[len("event: "):])
		case bytes.HasPrefix(line, []byte("data: ")):
			data = append(data, line[len("data: "):]...)
		}
	}
	if data == nil {
		return
	}
	updated := false
	switch event {
	case "message_start":
		var probe struct {
			Message struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(data, &probe); err == nil {
			u.inputTokens = probe.Message.Usage.InputTokens
			u.outputTokens = probe.Message.Usage.OutputTokens
			updated = true
		}
	case "message_delta":
		// delta usage is cumulative output tokens for this turn.
		var probe struct {
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &probe); err == nil && probe.Usage.OutputTokens > 0 {
			u.outputTokens = probe.Usage.OutputTokens
			updated = true
		}
	}
	if updated {
		u.sseSeen = true
		u.onUsage(u.inputTokens, u.outputTokens)
	}
}

func (u *usageTap) parseJSONLocked() {
	var probe struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(u.sseBuf, &probe); err == nil {
		u.inputTokens = probe.Usage.InputTokens
		u.outputTokens = probe.Usage.OutputTokens
	}
}
