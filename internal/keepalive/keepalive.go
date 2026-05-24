// Package keepalive implements ADR 0013's extended-cache keep-alive
// injection. Opt-in via `kete proxy --extended-cache` or
// KETE_EXTENDED_CACHE=true.
//
// On every successful forwarded request we stash the raw body + headers
// in this session map. A background ticker runs once a minute; for
// each session that's been idle past the 4-minute threshold (and is
// still within the 2-per-period cap), we send a keep-alive: the
// stashed bytes plus a single appended user message `,{"role":"user",
// "content":"."}` byte-spliced before the messages array's closing `]`.
//
// The prefix bytes (everything up to the splice point) are identical
// to the original request — that's what keeps the Anthropic prompt
// cache hot.
package keepalive

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/dreamware-nz/kete/internal/inject"
)

const (
	idleThreshold      = 4 * time.Minute
	maxIdleTotal       = 10 * time.Minute
	tickInterval       = 60 * time.Second
	maxPerIdlePeriod   = 2
)

// keepalivePayload is the byte-identical-shape we splice into messages.
// Matches the TS implementation exactly.
var keepalivePayload = []byte(`{"role":"user","content":"."}`)

// session is one cached request waiting for keep-alives.
type session struct {
	rawBody     []byte
	headers     http.Header
	upstreamURL string
	lastForward time.Time
	keepalives  int // count within the current idle period
}

// Manager runs the keepalive loop.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
	client   *http.Client
	stop     chan struct{}
}

// NewManager wires a manager but does not start it. Call Start.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*session),
		client:   &http.Client{Timeout: 30 * time.Second},
		stop:     make(chan struct{}),
	}
}

// Stash is called after every successful forward. sessionID is a
// per-conversation key (we use the project_path; one session per
// project is fine for v1).
func (m *Manager) Stash(sessionID string, rawBody []byte, headers http.Header, upstreamURL string) {
	body := make([]byte, len(rawBody))
	copy(body, rawBody)
	hdr := headers.Clone()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = &session{
		rawBody:     body,
		headers:     hdr,
		upstreamURL: upstreamURL,
		lastForward: time.Now(),
		keepalives:  0,
	}
}

// Start launches the background ticker. Stop with Close.
func (m *Manager) Start() {
	go m.loop()
}

// Close terminates the ticker.
func (m *Manager) Close() {
	close(m.stop)
}

func (m *Manager) loop() {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			m.tick()
		}
	}
}

func (m *Manager) tick() {
	m.tickAt(time.Now())
}

// tickAt is the testable seam — drives the same logic but with a
// caller-supplied time so tests don't have to wait 4 minutes.
func (m *Manager) tickAt(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		idle := now.Sub(s.lastForward)
		if idle > maxIdleTotal {
			delete(m.sessions, id)
			continue
		}
		if idle < idleThreshold {
			continue
		}
		if s.keepalives >= maxPerIdlePeriod {
			continue
		}
		s.keepalives++
		go m.fire(s)
	}
}

func (m *Manager) fire(s *session) {
	body, err := inject.AtMessages(s.rawBody, keepalivePayload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		s.upstreamURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.ContentLength = int64(len(body))
	for k, vs := range s.headers {
		req.Header[k] = vs
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
