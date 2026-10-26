//go:build integration

package natsrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn/authntest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// These tests are spike S5 of the plan for NATS: the queue group runs each request once, the
// pool bounds concurrency, deadlines are kept, and the client tells a timeout from no responders.

type replica struct {
	name    string
	handled sync.Map
	running atomic.Int32
	peak    atomic.Int32
	release chan struct{}
}

func startReplica(t *testing.T, url, name string, concurrency int) *replica {
	t.Helper()
	r := &replica{name: name, release: make(chan struct{})}
	router := rpc.NewRouter()
	router.Handle("v1.whoami", func(ctx context.Context, _ rpc.Request) (any, error) {
		id := requestid.From(ctx)
		if prev, loaded := r.handled.LoadOrStore(id, name); loaded {
			return nil, fmt.Errorf("request %s ran twice, first on %v", id, prev)
		}
		return map[string]string{"replica": name}, nil
	})
	router.Handle("v1.block", func(ctx context.Context, _ rpc.Request) (any, error) {
		n := r.running.Add(1)
		defer r.running.Add(-1)
		for {
			p := r.peak.Load()
			if n <= p || r.peak.CompareAndSwap(p, n) {
				break
			}
		}
		select {
		case <-r.release:
			return map[string]string{"replica": name}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := New(Options{
		URL: url, QueueGroup: "tbp-api", Concurrency: concurrency, ShutdownTimeout: 2 * time.Second,
		Router: router, Dispatcher: rpc.NewDispatcher(router, 30*time.Second, logger, authntest.Default()), Logger: logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("%s: Run = %v", name, err)
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for srv.Ready(context.Background()) != nil {
		if time.Now().After(deadline) {
			t.Fatal("server not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// micro registers its endpoints right after Run starts.
	time.Sleep(200 * time.Millisecond)
	return r
}

func dialClient(t *testing.T, url string) *Client {
	t.Helper()
	c, err := Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

// call sends a request with no envelope and decodes the reply.
func call(c *Client, method string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	answer, err := c.Call(ctx, method, rpc.Envelope{}, rpc.Headers{RequestID: requestid.New()})
	if err != nil {
		return err
	}
	var data map[string]string
	return rpc.DecodeReply(answer.Body, &data)
}

func TestNATSRPC(t *testing.T) {
	url := devstack.StartNATS(t)

	t.Run("no responders is not a timeout", func(t *testing.T) {
		client := dialClient(t, url)
		if err := call(client, "v1.whoami", 2*time.Second); !errors.Is(err, rpc.ErrNoResponders) {
			t.Fatalf("err = %v, want rpc.ErrNoResponders", err)
		}
	})

	t.Run("each request runs once across two replicas", func(t *testing.T) {
		a := startReplica(t, url, "a", 4)
		b := startReplica(t, url, "b", 4)
		client := dialClient(t, url)
		var wg sync.WaitGroup
		errs := make(chan error, 40)
		for range 40 {
			wg.Go(func() {
				if err := call(client, "v1.whoami", 10*time.Second); err != nil {
					errs <- err
				}
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		total := 0
		for _, r := range []*replica{a, b} {
			count := 0
			r.handled.Range(func(_, _ any) bool { count++; return true })
			if count == 0 {
				t.Errorf("replica %s served nothing; the queue group is not shared", r.name)
			}
			total += count
		}
		if total != 40 {
			t.Fatalf("%d requests handled, want 40 exactly once", total)
		}
	})

	t.Run("the pool bounds concurrency and a slow reply is a timeout", func(t *testing.T) {
		r := startReplica(t, url, "bounded", 2)
		client := dialClient(t, url)
		results := make(chan error, 4)
		for range 4 {
			go func() {
				results <- call(client, "v1.block", 10*time.Second)
			}()
		}
		time.Sleep(500 * time.Millisecond)
		if got := r.running.Load(); got != 2 {
			t.Fatalf("%d running, want 2 (RPC_CONCURRENCY)", got)
		}
		if err := call(client, "v1.block", 300*time.Millisecond); !errors.Is(err, rpc.ErrTimeout) {
			t.Fatalf("short deadline = %v, want rpc.ErrTimeout", err)
		}
		close(r.release)
		for range 4 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		if r.peak.Load() != 2 {
			t.Errorf("peak %d, want 2", r.peak.Load())
		}
	})

	t.Run("errors keep the envelope and set micro's headers", func(t *testing.T) {
		startReplica(t, url, "errors", 1)
		client := dialClient(t, url)
		msgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		answer, err := client.Call(msgCtx, "v1.whoami", rpc.Envelope{}, rpc.Headers{RequestID: "req-1"})
		if err != nil {
			t.Fatal(err)
		}
		if answer.RequestID != "req-1" {
			t.Errorf("request ID %q, want req-1 echoed", answer.RequestID)
		}
		raw, err := client.nc.Request(SubjectPrefix+".whoami", []byte(`{"params":`), 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if raw.Header.Get("Nats-Service-Error-Code") != "400" || raw.Header.Get("Nats-Service-Error") != "invalid-request" {
			t.Fatalf("headers = %v", raw.Header)
		}
		var rpcErr *rpc.Error
		if err := rpc.DecodeReply(raw.Data, nil); !errors.As(err, &rpcErr) || rpcErr.Code != "INVALID" {
			t.Fatalf("reply = %s", raw.Data)
		}
	})
}
