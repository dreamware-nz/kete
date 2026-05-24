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
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

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
	cfg   Config
	store *store.DB
	http  *http.Server
}

// NewServer wires the chi router and an http.Server. It does not bind
// until Run is called.
func NewServer(cfg Config, db *store.DB) *Server {
	r := chi.NewRouter()
	s := &Server{cfg: cfg, store: db}
	r.Get("/health", s.handleHealth)
	r.NotFound(s.handleNotFound)

	s.http = &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
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
	return s.http.Shutdown(ctx)
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
