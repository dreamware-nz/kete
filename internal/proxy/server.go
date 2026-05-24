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
	"time"

	"github.com/dreamware-nz/kete/internal/adapter"
	"github.com/dreamware-nz/kete/internal/adapter/anthropic"
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
	cfg      Config
	store    *store.DB
	http     *http.Server
	adapters map[Upstream]adapter.Wire
	capture  *capture
}

// NewServer wires the chi router and an http.Server. It does not bind
// until Run is called.
func NewServer(cfg Config, db *store.DB) *Server {
	r := chi.NewRouter()
	r.Use(timeoutMiddleware(cfg.RequestTimeout))
	r.Use(maxBodyMiddleware(cfg.MaxBodyBytes))

	s := &Server{
		cfg: cfg, store: db,
		adapters: defaultAdapters(),
		capture:  newCapture(db),
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

// defaultAdapters wires the three upstreams. cc-proxy and Bedrock land
// in plans 013 and 012; for now those two slots are nil and selecting
// them returns 501 from handleMessages.
func defaultAdapters() map[Upstream]adapter.Wire {
	return map[Upstream]adapter.Wire{
		UpstreamAnthropic: anthropic.New(),
	}
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

	// Inject prior memory before forwarding. Failure is non-fatal:
	// brief 002 says enrichment never blocks forwarding.
	injected, err := injectMemory(r.Context(), s.store, project, rawBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inject(%s): %v\n", project, err)
		injected = rawBody
	}

	if err := ad.Forward(r.Context(), injected, SanitiseHeaders(r.Header), w); err != nil {
		fmt.Fprintf(os.Stderr, "forward(%s): %v\n", up, err)
	}

	// Capture the *original* body (pre-injection) so the captured
	// reasoning trace doesn't include kete's own injections.
	s.capture.Record(project, "proxy", rawBody)
}
