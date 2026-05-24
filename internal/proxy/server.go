// Package proxy is kete's local HTTP proxy.
//
// One process per `kete proxy`. Binds 127.0.0.1:8080 by default; routes
// POST /v1/messages to the configured upstream byte-exact (ADR 0006),
// returns 200 on GET /health, 404 on everything else.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dreamware-nz/kete/internal/adapter"
	"github.com/dreamware-nz/kete/internal/adapter/anthropic"
	"github.com/dreamware-nz/kete/internal/adapter/bedrock"
	"github.com/dreamware-nz/kete/internal/adapter/ccproxy"
	"github.com/dreamware-nz/kete/internal/compact"
	"github.com/dreamware-nz/kete/internal/drift"
	"github.com/dreamware-nz/kete/internal/extract"
	"github.com/dreamware-nz/kete/internal/keepalive"
	"github.com/dreamware-nz/kete/internal/store"
	"github.com/go-chi/chi/v5"
)

// Config is the resolved proxy server configuration. Built from env in
// LoadConfig; passed by value to NewServer.
type Config struct {
	Host           string
	Port           int
	MaxBodyBytes   int64
	RequestTimeout time.Duration
	ExtendedCache  bool
}

const (
	defaultHost           = "127.0.0.1"
	defaultPort           = 8080
	defaultMaxBodyBytes   = 10 * 1024 * 1024 // 10 MB
	defaultRequestTimeout = 5 * time.Minute
)

// LoadConfig builds a Config from KETE_HOST / KETE_PORT, falling back
// to the defaults from brief 002.
func LoadConfig() (Config, error) {
	c := Config{
		Host:           defaultHost,
		Port:           defaultPort,
		MaxBodyBytes:   defaultMaxBodyBytes,
		RequestTimeout: defaultRequestTimeout,
	}
	if h := os.Getenv("KETE_HOST"); h != "" {
		c.Host = h
	}
	if p := os.Getenv("KETE_PORT"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return c, fmt.Errorf("KETE_PORT=%q: %w", p, err)
		}
		c.Port = n
	}
	return c, nil
}

// Server holds the running HTTP server's state. One per process.
type Server struct {
	cfg       Config
	store     *store.DB
	http      *http.Server
	adapters  map[Upstream]adapter.Wire
	capture   *capture
	driftHook *driftHook
	driftSt   *drift.State
	extractor *extract.Client // shared with capture; used for drift scoring
	keepalive *keepalive.Manager
	compactor *compactSessions

	mu          sync.Mutex
	corrections map[string]string // project -> pending correction text
}

// NewServer wires the chi router and an http.Server. It does not bind
// until Run is called.
func NewServer(cfg Config, db *store.DB) *Server {
	r := chi.NewRouter()
	r.Use(timeoutMiddleware(cfg.RequestTimeout))
	r.Use(maxBodyMiddleware(cfg.MaxBodyBytes))

	s := &Server{
		cfg: cfg, store: db,
		adapters:    defaultAdapters(),
		capture:     newCapture(db),
		driftHook:   newDriftHook(),
		driftSt:     drift.NewState(),
		compactor:   newCompactSessions(),
		corrections: make(map[string]string),
	}
	// Capture builds its own extractor; reuse it for drift scoring so
	// we don't open a second client. SetExtractor is exported for
	// tests; the field stays unexported.
	s.extractor = s.capture.extractor
	if cfg.ExtendedCache {
		s.keepalive = keepalive.NewManager()
		s.keepalive.Start()
	}
	r.Get("/health", s.handleHealth)
	r.Post("/v1/messages", s.handleMessages)
	r.NotFound(s.handleNotFound)

	s.http = &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

// timeoutMiddleware applies a per-request context timeout. Bodies that
// take longer than d to forward will see ctx.Err() == DeadlineExceeded.
func timeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// maxBodyMiddleware caps request bodies. Reads past the cap return 413.
// We deliberately wrap r.Body rather than buffering up-front so the
// passthrough forwarder (phase 5) can stream and still get a 413 on
// overflow without holding the whole body in RAM.
func maxBodyMiddleware(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}

// Run starts the listener and blocks until ctx is cancelled or
// ListenAndServe returns. Returns nil on graceful shutdown.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.http.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		return s.shutdown()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := s.http.Shutdown(ctx)
	if s.keepalive != nil {
		s.keepalive.Close()
	}
	s.capture.Wait()
	return err
}

// Addr returns the bound address (host:port). Useful for tests.
func (s *Server) Addr() string {
	return s.http.Addr
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"Not found"}`))
}

// defaultAdapters wires the three upstreams. Adapters that fail to
// configure (no AWS region, no cc-proxy key) are left nil; selecting
// them returns 501 from handleMessages so the user sees a clear error.
func defaultAdapters() map[Upstream]adapter.Wire {
	out := map[Upstream]adapter.Wire{
		UpstreamAnthropic: anthropic.New(),
	}
	if a, err := bedrock.New(context.Background()); err == nil {
		out[UpstreamBedrock] = a
	}
	if a, err := ccproxy.New(); err == nil {
		out[UpstreamCCProxy] = a
	}
	return out
}

// handleMessages routes a /v1/messages request through the upstream
// selector and the chosen adapter. The body is read once into a
// []byte and forwarded byte-exact (ADR 0006).
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "request body exceeds limit", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	up, err := SelectUpstream(r.Header, rawBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ad, ok := s.adapters[up]
	if !ok || ad == nil {
		http.Error(w, fmt.Sprintf("upstream %q not yet implemented", up),
			http.StatusNotImplemented)
		return
	}

	project := projectPath()

	// If a prior turn crossed the clear threshold, rewrite the body
	// to drop the conversation in favour of the structured summary.
	// This is the deliberate ADR 0006 exception: compaction *is* a
	// re-marshal, by design.
	if s.compactor.drainPending(project) {
		if summary, ok := s.compactor.cache.Get(project); ok && summary != nil {
			next := compact.LastUserPrompt(rawBody)
			if rewritten, err := compact.Apply(rawBody, summary, next); err == nil {
				rawBody = rewritten
			}
		}
	}

	// Inject prior memory before forwarding. Failure is non-fatal:
	// brief 002 says enrichment never blocks forwarding.
	injected, err := injectMemory(r.Context(), s.store, project, rawBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inject(%s): %v\n", project, err)
		injected = rawBody
	}

	// If a previous turn produced a correction, splice it ahead of
	// memory. One correction per request; consume on inject.
	if correction := s.consumeCorrection(project); correction != "" {
		if b, err := injectCorrectionPayload(injected, correction); err == nil {
			injected = b
		}
	}

	// Wrap w with a usage tap so we observe Anthropic-shaped token
	// counts as the response streams back. cb is nil when there's no
	// extractor (no compaction can happen without one), making the
	// tap a transparent passthrough.
	tap := newUsageTap(w, s.usageCallback(project, rawBody))
	defer tap.Done()

	sanitised := SanitiseHeaders(r.Header)
	if err := ad.Forward(r.Context(), injected, sanitised, tap); err != nil {
		fmt.Fprintf(os.Stderr, "forward(%s): %v\n", up, err)
	}

	// Capture the *original* body (pre-injection) so the captured
	// reasoning trace doesn't include kete's own injections.
	s.capture.Record(project, "proxy", rawBody)

	// Drift check every Nth request; result feeds the next request's
	// correction queue so the hot path stays cheap-ish.
	if s.driftHook.Tick() {
		go s.scoreAndQueueCorrection(project, rawBody)
	}

	// Stash for keepalive if enabled. We use the project as the
	// session id (one session per project for v1).
	if s.keepalive != nil && up == UpstreamAnthropic {
		s.keepalive.Stash(project, rawBody, sanitised, anthropicURLFromAdapter(ad))
	}
}

// usageCallback builds the per-request closure passed to the usage
// tap. nil when there's no extractor (compaction can't run).
func (s *Server) usageCallback(project string, conversation []byte) func(int, int) {
	if s.extractor == nil {
		return nil
	}
	convo := string(conversation)
	return func(in, out int) {
		s.compactor.observe(s.extractor, project, convo, in+out)
	}
}

// scoreAndQueueCorrection runs Haiku-based drift detection on the
// captured request and, if the level warrants, builds a correction
// and stashes it for the next request.
func (s *Server) scoreAndQueueCorrection(project string, rawBody []byte) {
	if s.extractor == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Goal lookup: most recent enriched task for this project.
	tasks, err := s.store.ListTasks(ctx, project)
	if err != nil || len(tasks) == 0 {
		return
	}
	var goal string
	for _, t := range tasks {
		if t.Goal != "" {
			goal = t.Goal
			break
		}
	}
	if goal == "" {
		return
	}
	score, level, err := drift.ScoreAction(ctx, s.extractor, goal, string(rawBody))
	if err != nil {
		return
	}
	s.driftSt.Record(project, level)
	// Persist regardless of level; goal selection above already gave us
	// a task id but we'll just attach to the latest captured task.
	latest := tasks[0].ID
	correction := ""
	if level != drift.LevelNone {
		correction, _ = drift.BuildCorrection(ctx, s.extractor, goal, string(rawBody), level)
	}
	_ = drift.Persist(ctx, s.store, latest, score, level, correction)
	if correction != "" {
		s.queueCorrection(project, correction)
	}
}

func (s *Server) queueCorrection(project, text string) {
	s.mu.Lock()
	s.corrections[project] = text
	s.mu.Unlock()
}

func (s *Server) consumeCorrection(project string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	text := s.corrections[project]
	delete(s.corrections, project)
	return text
}

// anthropicURLFromAdapter pulls the BaseURL out of the anthropic
// adapter so the keepalive manager has a real URL to dial. Falls
// back to the public endpoint.
func anthropicURLFromAdapter(ad adapter.Wire) string {
	if a, ok := ad.(*anthropic.Adapter); ok && a.BaseURL != "" {
		return a.BaseURL + "/v1/messages"
	}
	return anthropic.DefaultBaseURL + "/v1/messages"
}
