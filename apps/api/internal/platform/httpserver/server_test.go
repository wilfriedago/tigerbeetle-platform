package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunServesAndShutsDown(t *testing.T) {
	t.Parallel()
	// Reserve a free port, then release it for the server.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			Addr:            addr,
			Handler:         http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
			ShutdownTimeout: time.Second,
			Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()

	client := &http.Client{Timeout: time.Second}
	var resp *http.Response
	for range 50 {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/", http.NoBody)
		resp, err = client.Do(req)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server never answered: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRunListenError(t *testing.T) {
	t.Parallel()
	err := Run(context.Background(), Options{
		Addr:            "256.256.256.256:1",
		Handler:         http.NotFoundHandler(),
		ShutdownTimeout: time.Second,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil {
		t.Fatal("expected a listen error")
	}
}
