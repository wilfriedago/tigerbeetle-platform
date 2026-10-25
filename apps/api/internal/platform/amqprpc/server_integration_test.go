//go:build integration

package amqprpc

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

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// These tests are spike S5 of the plan for AMQP: competing consumers run each request once,
// the pool bounds concurrency, deadlines are kept, and a dropped connection is redialled.

const queue = "tbp.rpc.test"

type replica struct {
	name    string
	server  *Server
	cancel  context.CancelFunc
	done    chan error
	handled sync.Map // request ID -> replica name
}

type counters struct {
	running, peak atomic.Int32
	release       chan struct{}
}

func (c *counters) enter() {
	n := c.running.Add(1)
	for {
		p := c.peak.Load()
		if n <= p || c.peak.CompareAndSwap(p, n) {
			return
		}
	}
}

func startReplica(t *testing.T, url, name string, concurrency int, c *counters) *replica {
	t.Helper()
	r := &replica{name: name, done: make(chan error, 1)}
	router := rpc.NewRouter()
	router.Handle("v1.whoami", func(ctx context.Context, _ rpc.Request) (any, error) {
		id := requestid.From(ctx)
		if prev, loaded := r.handled.LoadOrStore(id, name); loaded {
			return nil, fmt.Errorf("request %s ran twice, first on %v", id, prev)
		}
		return map[string]string{"replica": name}, nil
	})
	router.Handle("v1.block", func(ctx context.Context, _ rpc.Request) (any, error) {
		c.enter()
		defer c.running.Add(-1)
		select {
		case <-c.release:
			return map[string]string{"replica": name}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r.server = New(Options{
		URL: url, Queue: queue, Concurrency: concurrency, ShutdownTimeout: 2 * time.Second,
		Dispatcher: rpc.NewDispatcher(router, 30*time.Second, logger), Logger: logger,
	})
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() { r.done <- r.server.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-r.done; err != nil {
			t.Errorf("%s: Run = %v", name, err)
		}
	})
	waitReady(t, r.server)
	return r
}

func waitReady(t *testing.T, s *Server) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := s.Ready(ctx)
		cancel()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server not ready: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func dialClient(t *testing.T, url string) *Client {
	t.Helper()
	c, err := Dial(url, queue)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func call(t *testing.T, c *Client, method string, timeout time.Duration) (map[string]string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	answer, err := c.Call(ctx, method, rpc.Envelope{}, "", requestid.New())
	if err != nil {
		return nil, err
	}
	var data map[string]string
	err = rpc.DecodeReply(answer.Body, &data)
	return data, err
}

func TestAMQPRPC(t *testing.T) {
	url := devstack.StartRabbitMQ(t)

	t.Run("each request runs once across two replicas", func(t *testing.T) {
		a := startReplica(t, url, "a", 4, &counters{})
		b := startReplica(t, url, "b", 4, &counters{})
		client := dialClient(t, url)
		var wg sync.WaitGroup
		served := sync.Map{}
		errs := make(chan error, 40)
		for range 40 {
			wg.Go(func() {
				data, err := call(t, client, "v1.whoami", 10*time.Second)
				if err != nil {
					errs <- err
					return
				}
				n, _ := served.LoadOrStore(data["replica"], new(atomic.Int32))
				n.(*atomic.Int32).Add(1)
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
				t.Errorf("replica %s served nothing; the queue is not shared", r.name)
			}
			total += count
		}
		if total != 40 {
			t.Fatalf("%d requests handled, want 40 exactly once", total)
		}
	})

	t.Run("the pool bounds concurrency and the deadline drops a waiting request", func(t *testing.T) {
		c := &counters{release: make(chan struct{})}
		startReplica(t, url, "bounded", 2, c)
		client := dialClient(t, url)
		results := make(chan error, 4)
		for range 4 {
			go func() {
				_, err := call(t, client, "v1.block", 10*time.Second)
				results <- err
			}()
		}
		time.Sleep(500 * time.Millisecond)
		if got := c.running.Load(); got != 2 {
			t.Fatalf("%d running, want 2 (RPC_CONCURRENCY)", got)
		}
		// A request with a short deadline waits behind the others; RabbitMQ drops it when its
		// expiration passes, so it never runs and the client sees a timeout.
		if _, err := call(t, client, "v1.block", 300*time.Millisecond); !errors.Is(err, rpc.ErrTimeout) {
			t.Fatalf("short deadline = %v, want rpc.ErrTimeout", err)
		}
		close(c.release)
		for range 4 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		if c.peak.Load() != 2 {
			t.Errorf("peak %d, want 2", c.peak.Load())
		}
	})

	t.Run("a request past its deadline is answered without running", func(t *testing.T) {
		c := &counters{release: make(chan struct{})}
		close(c.release)
		startReplica(t, url, "late", 1, c)
		client := dialClient(t, url)
		answer := rawCall(t, client, "v1.block", amqp091.Table{rpc.HeaderDeadline: rpc.DeadlineHeader(time.Now().Add(-time.Second))})
		var data map[string]string
		var rpcErr *rpc.Error
		if err := rpc.DecodeReply(answer.Body, &data); !errors.As(err, &rpcErr) || rpcErr.Reason != "deadline-exceeded" || rpcErr.Code != "UNAVAILABLE" {
			t.Fatalf("reply = %s", answer.Body)
		}
		if c.peak.Load() != 0 {
			t.Error("the handler ran")
		}
	})

	t.Run("a dropped connection is redialled", func(t *testing.T) {
		r := startReplica(t, url, "redial", 2, &counters{})
		client := dialClient(t, url)
		if _, err := call(t, client, "v1.whoami", 5*time.Second); err != nil {
			t.Fatal(err)
		}
		if err := r.server.supervisor.Connection().Close(); err != nil {
			t.Fatal(err)
		}
		waitReady(t, r.server)
		if _, err := call(t, client, "v1.whoami", 10*time.Second); err != nil {
			t.Fatalf("after redial: %v", err)
		}
	})
}

// rawCall publishes a request with the given headers and no expiration, as a client that set
// x-deadline but whose message still got through.
func rawCall(t *testing.T, c *Client, method string, headers amqp091.Table) rpc.Answer {
	t.Helper()
	correlation := requestid.New()
	waiter := make(chan amqp091.Delivery, 1)
	c.mu.Lock()
	c.pending[correlation] = waiter
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := c.ch.PublishWithContext(ctx, Exchange, c.queue, false, false, amqp091.Publishing{
		Type: method, CorrelationId: correlation, ReplyTo: directReplyTo, Headers: headers,
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case d := <-waiter:
		return rpc.Answer{Body: d.Body}
	case <-ctx.Done():
		t.Fatal("no reply")
		return rpc.Answer{}
	}
}
