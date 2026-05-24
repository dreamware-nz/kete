package proxy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	capturepkg "github.com/dreamware-nz/kete/internal/capture"
	"github.com/dreamware-nz/kete/internal/extract"
	"github.com/dreamware-nz/kete/internal/store"
	"github.com/google/uuid"
)

// capture asynchronously writes a raw `tasks` row after a request
// has finished, then (if an extractor is configured) enriches the row
// with goal / decisions / files_touched via Haiku.
//
// Errors are logged-and-swallowed. Capture must never block or break
// the proxy hot path.
type capture struct {
	store     *store.DB
	extractor *extract.Client // optional; nil means "raw only"
	wg        sync.WaitGroup
}

// newCapture builds a capture that always writes raw rows. It tries
// to construct an extractor; if ANTHROPIC_API_KEY is missing, the
// extractor stays nil and rows ship raw — the proxy still works,
// just without enrichment.
func newCapture(db *store.DB) *capture {
	c := &capture{store: db}
	if ex, err := extract.NewClient(); err == nil {
		c.extractor = ex
	}
	return c
}

// SetExtractor replaces the extractor (used in tests to inject a
// stubbed Anthropic endpoint). nil disables enrichment.
func (c *capture) SetExtractor(ex *extract.Client) {
	c.extractor = ex
}

// minCaptureBytes filters out plumbing requests that are not "what
// the user is working on": Crush keepalive pings (~150 bytes,
// {"messages":[{"content":"reply with PONG"}],"max_tokens":32}),
// session-title generation, autocomplete, and similar small-model
// utility traffic. Real coding-turn bodies carry the system prompt,
// tools, and content blocks and easily clear several KiB even on
// the first turn. 2 KiB cleanly separates the two populations.
//
// Override via KETE_CAPTURE_MIN_BYTES.
const minCaptureBytes = 2048

// Record schedules a raw write, then an enrichment pass. Safe to
// call from a request handler. rawBody is copied.
func (c *capture) Record(project, source string, rawBody []byte) {
	if c.store == nil || project == "" {
		return
	}
	if len(rawBody) < envInt("KETE_CAPTURE_MIN_BYTES", minCaptureBytes) {
		return
	}
	body := make([]byte, len(rawBody))
	copy(body, rawBody)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		id := uuid.NewString()
		t := &store.Task{
			ID:             id,
			ProjectPath:    project,
			Source:         source,
			ReasoningTrace: capturepkg.ExtractConversation(body),
		}
		if err := c.store.CreateTask(context.Background(), t); err != nil {
			fmt.Fprintf(os.Stderr, "capture: create %s: %v\n", id, err)
			return
		}
		c.enrich(id, body)
	}()
}

// enrich runs ExtractTask against the captured body and updates the
// row in place. Failure is logged and swallowed — the raw row stays.
//
// Bounded to 60 s so a slow Haiku doesn't pin a goroutine forever
// during shutdown.
func (c *capture) enrich(taskID string, body []byte) {
	if c.extractor == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	clipped := capturepkg.ExtractConversation(body)
	out, err := c.extractor.ExtractTask(ctx, clipped)
	if err != nil {
		// Network errors / non-JSON responses both land here. Don't
		// noise the log under context cancellation (shutdown).
		if !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "capture: enrich %s: %v\n", taskID, err)
		}
		return
	}
	// Map extract.Decision -> store.Decision (same shape, different
	// package).
	decs := make([]store.Decision, len(out.Decisions))
	for i, d := range out.Decisions {
		decs[i] = store.Decision{Choice: d.Choice, Rationale: d.Rationale}
	}
	if err := c.store.UpdateTask(ctx, taskID, out.Goal, decs, out.FilesTouched, clipped); err != nil {
		fmt.Fprintf(os.Stderr, "capture: update %s: %v\n", taskID, err)
	}
}

// Wait blocks until in-flight captures finish. Used by the server's
// graceful shutdown path so we don't lose rows on SIGINT.
func (c *capture) Wait() {
	c.wg.Wait()
}
