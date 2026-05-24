package extract

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// SendWithRetry wraps Send with exponential backoff. 5xx and 429
// retry up to maxAttempts; everything else fails fast. The caller's
// context is propagated; ctx cancellation aborts immediately.
func (c *Client) SendWithRetry(ctx context.Context, req Request) (*Response, error) {
	const (
		maxAttempts = 3
		baseDelay   = 200 * time.Millisecond
	)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := c.Send(ctx, req)
		if err == nil {
			return resp, nil
		}
		var httpErr *HTTPError
		if errors.As(err, &httpErr) && !httpErr.IsRetryable() {
			return nil, err
		}
		lastErr = err
		if attempt == maxAttempts {
			break
		}
		// Exponential backoff: 200ms, 400ms.
		delay := baseDelay * time.Duration(1<<(attempt-1))
		slog.Warn("extract retry",
			"attempt", attempt, "max", maxAttempts,
			"delay", delay, "err", err.Error())
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}
