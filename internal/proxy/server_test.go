package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// startTestServer starts a Server on :0 and returns the bound URL plus
// a stop func.
func startTestServer(t *testing.T, cfg Config) (string, func()) {
	t.Helper()
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	cfg.MaxBodyBytes = defaultMaxBodyBytes
	cfg.RequestTimeout = defaultRequestTimeout

	// Bind on :0 ourselves to discover the port.
	ln, err := net.Listen("tcp", net.JoinHostPort(cfg.Host, "0"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Port = ln.Addr().(*net.TCPAddr).Port

	srv := NewServer(cfg, nil)
	go func() {
		_ = srv.http.Serve(ln)
	}()

	url := "http://" + ln.Addr().String()

	// Block on /health until the listener is actually serving.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	stop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_ = srv.http.Shutdown(ctx)
	}
	return url, stop
}

func TestHealthOK(t *testing.T) {
	url, stop := startTestServer(t, Config{})
	defer stop()

	resp, err := http.Get(url + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status=%d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"ok"`) {
		t.Errorf("body=%q, want contain ok", body)
	}
}

func TestNotFound(t *testing.T) {
	url, stop := startTestServer(t, Config{})
	defer stop()

	resp, err := http.Get(url + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("status=%d, want 404", resp.StatusCode)
	}
}
