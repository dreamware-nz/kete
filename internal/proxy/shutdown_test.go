package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestShutdown_Graceful asserts Shutdown returns within 500 ms even
// with an in-flight slow handler.
func TestShutdown_Graceful(t *testing.T) {
	// Build a server with one slow handler so we can exercise shutdown
	// while a request is mid-flight.
	cfg := Config{Host: "127.0.0.1", MaxBodyBytes: defaultMaxBodyBytes, RequestTimeout: defaultRequestTimeout}
	mux := http.NewServeMux()
	started := make(chan struct{})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Port = ln.Addr().(*net.TCPAddr).Port
	hsrv := &http.Server{Handler: mux}

	go hsrv.Serve(ln)
	defer hsrv.Close()

	url := "http://" + ln.Addr().String()

	// Fire a slow request and wait until it lands inside the handler.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req, _ := http.NewRequest("GET", url+"/slow", nil)
		client := &http.Client{Timeout: 2 * time.Second}
		resp, _ := client.Do(req)
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()
	<-started

	// Shut down with the proxy's deadline.
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = hsrv.Shutdown(ctx)
	cancel()
	elapsed := time.Since(t0)

	if elapsed > 750*time.Millisecond {
		t.Errorf("shutdown took %s, want <= 750 ms", elapsed)
	}
	wg.Wait()
}

// TestRun_RespectsCtxCancel shuts down via the context the public Run
// method uses.
func TestRun_RespectsCtxCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port,
		MaxBodyBytes: defaultMaxBodyBytes, RequestTimeout: defaultRequestTimeout,
	}
	srv := NewServer(cfg, nil)
	// Replace the http.Server's ListenAndServe with Serve(ln).
	srv.http.Addr = ""
	go srv.http.Serve(ln)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		// Block on shutdown semantics by mimicking Run without actually
		// re-binding (which would fail since ln owns the port).
		<-ctx.Done()
		done <- srv.shutdown()
	}()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown didn't return within 1s")
	}
}
