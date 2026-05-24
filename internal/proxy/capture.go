package proxy

import (
	"context"
	"sync"

	"github.com/dreamware-nz/kete/internal/store"
	"github.com/google/uuid"
)

// captureRaw asynchronously writes a raw `tasks` row after a request
// has finished. Extraction (plan 011) fills in goal/decisions later;
// for now we just capture the raw bytes so we never lose them.
//
// Errors are logged-and-swallowed. Capture must never block or break
// the proxy hot path.
type capture struct {
	store *store.DB
	wg    sync.WaitGroup
}

func newCapture(db *store.DB) *capture {
	return &capture{store: db}
}

// Record schedules a raw write. Safe to call from a request handler.
// rawBody is copied; the caller can reuse the slice immediately.
func (c *capture) Record(project, source string, rawBody []byte) {
	if c.store == nil || project == "" {
		return
	}
	body := make([]byte, len(rawBody))
	copy(body, rawBody)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		t := &store.Task{
			ID:             uuid.NewString(),
			ProjectPath:    project,
			Source:         source,
			ReasoningTrace: string(body),
		}
		_ = c.store.CreateTask(context.Background(), t)
	}()
}

// Wait blocks until in-flight captures finish. Used by the server's
// graceful shutdown path so we don't lose rows on SIGINT.
func (c *capture) Wait() {
	c.wg.Wait()
}
