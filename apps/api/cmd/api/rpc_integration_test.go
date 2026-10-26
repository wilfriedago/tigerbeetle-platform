//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/bootstrap"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/amqprpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/natsrpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// caller is what the AMQP and NATS clients have in common.
type caller interface {
	Call(ctx context.Context, method string, env rpc.Envelope, h rpc.Headers) (rpc.Answer, error)
}

type rpcStack struct {
	t     *testing.T
	amqp  *amqprpc.Client
	nats  *natsrpc.Client
	rest  *httptest.Server
	apps  []*bootstrap.App
	queue string
	auth  testAuth
}

// newRPCStack starts the stores and both brokers, then two API replicas with AMQP and NATS RPC
// on, as two processes of one deployment would be.
func newRPCStack(t *testing.T) *rpcStack {
	t.Helper()
	cfg := config.Config{
		Addr:                  "127.0.0.1:0",
		PostgresURL:           devstack.StartPostgres(t),
		TigerBeetleAddresses:  []string{devstack.StartTigerBeetle(t)},
		MigrateOnStart:        true,
		RESTEnabled:           true,
		ShutdownTimeout:       5 * time.Second,
		ReadinessTimeout:      2 * time.Second,
		TigerBeetleTimeout:    10 * time.Second,
		ReconcilerGracePeriod: time.Minute,
		ReconcilerInterval:    time.Hour,
		AMQPRPCEnabled:        true,
		AMQPURL:               devstack.StartRabbitMQ(t),
		AMQPRPCQueue:          "tbp.rpc.v1",
		NATSRPCEnabled:        true,
		NATSURL:               devstack.StartNATS(t),
		NATSRPCQueueGroup:     "tbp-api",
		RPCConcurrency:        4,
		RPCTimeoutMax:         30 * time.Second,
	}
	s := &rpcStack{t: t, queue: cfg.AMQPRPCQueue}
	s.auth = trustTestIssuer(t, &cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for range 2 {
		app, err := bootstrap.Build(context.Background(), cfg, bootstrap.Service{Name: serviceName, Version: version}, logger)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- app.Run(ctx) }()
		t.Cleanup(func() {
			cancel()
			if err := <-done; err != nil {
				t.Errorf("run: %v", err)
			}
		})
		s.apps = append(s.apps, app)
	}
	s.rest = httptest.NewServer(s.auth.withCredentials(s.apps[0].Handler()))
	t.Cleanup(s.rest.Close)

	var err error
	if s.amqp, err = amqprpc.Dial(cfg.AMQPURL, cfg.AMQPRPCQueue); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.amqp.Close() })
	if s.nats, err = natsrpc.Dial(cfg.NATSURL); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.nats.Close)

	// Both replicas declare their topology when they connect; wait until both brokers answer.
	for _, c := range []caller{s.amqp, s.nats} {
		deadline := time.Now().Add(30 * time.Second)
		for {
			if err := s.call(c, "v1.getLiveness", rpc.Envelope{}, "", nil); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the RPC servers did not start")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return s
}

// call sends one request and decodes the reply data into out.
func (s *rpcStack) call(c caller, method string, env rpc.Envelope, key string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	answer, err := c.Call(ctx, method, env, s.auth.rpcHeaders(key))
	if err != nil {
		return err
	}
	return rpc.DecodeReply(answer.Body, out)
}

func TestBrokerRPCEndToEnd(t *testing.T) {
	s := newRPCStack(t)
	ledger := map[string]string{"ledger": "brokers"}

	var created struct{ Name string }
	if err := s.call(s.amqp, "v1.createLedger", rpc.Envelope{Params: ledger}, "", &created); err != nil || created.Name != "brokers" {
		t.Fatalf("createLedger over AMQP = %+v, %v", created, err)
	}

	write := rpc.Envelope{Params: ledger, Body: map[string]any{"postings": []any{posting("world", "users:1:wallet", "EUR/2", "1250")}}}
	var first struct{ ID string }
	if err := s.call(s.amqp, "v1.createTransaction", write, "broker-1", &first); err != nil || first.ID == "" {
		t.Fatalf("createTransaction over AMQP = %+v, %v", first, err)
	}

	t.Run("a write sent again replays on every transport", func(t *testing.T) {
		for name, c := range map[string]caller{"AMQP redelivery": s.amqp, "NATS": s.nats} {
			var again struct{ ID string }
			if err := s.call(c, "v1.createTransaction", write, "broker-1", &again); err != nil || again.ID != first.ID {
				t.Fatalf("%s replay = %+v, %v; want %s", name, again, err, first.ID)
			}
		}
		body, _ := json.Marshal(write.Body)
		req, _ := http.NewRequest(http.MethodPost, s.rest.URL+"/v1/ledgers/brokers/transactions", bytes.NewReader(body))
		req.Header.Set("Idempotency-Key", "broker-1")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var rest struct{ ID string }
		_ = json.NewDecoder(resp.Body).Decode(&rest)
		if resp.StatusCode != http.StatusCreated || rest.ID != first.ID {
			t.Fatalf("REST replay = %d %+v, want %s", resp.StatusCode, rest, first.ID)
		}
		var account struct {
			Balances []struct{ Balance string }
		}
		if err := s.call(s.nats, "v1.getAccount", rpc.Envelope{Params: map[string]string{"ledger": "brokers", "address": "users:1:wallet"}}, "", &account); err != nil {
			t.Fatal(err)
		}
		if len(account.Balances) != 1 || account.Balances[0].Balance != "1250" {
			t.Fatalf("balance after four sends of one write = %+v, want 1250 once", account.Balances)
		}
	})

	t.Run("writes spread over two replicas each apply once", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 40)
		for i := range 40 {
			c := caller(s.amqp)
			if i%2 == 1 {
				c = s.nats
			}
			wg.Go(func() {
				env := rpc.Envelope{Params: ledger, Body: map[string]any{"postings": []any{posting("world", "users:2:wallet", "EUR/2", "1")}}}
				if err := s.call(c, "v1.createTransaction", env, fmt.Sprintf("spread-%d", i), nil); err != nil {
					errs <- err
				}
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		var account struct {
			Balances []struct{ Balance string }
		}
		if err := s.call(s.amqp, "v1.getAccount", rpc.Envelope{Params: map[string]string{"ledger": "brokers", "address": "users:2:wallet"}}, "", &account); err != nil {
			t.Fatal(err)
		}
		if account.Balances[0].Balance != "40" {
			t.Fatalf("balance = %s, want 40", account.Balances[0].Balance)
		}
	})

	t.Run("a decline is an error envelope", func(t *testing.T) {
		env := rpc.Envelope{Params: ledger, Body: map[string]any{"postings": []any{
			posting("world", "users:3:wallet", "EUR/2", "5"),
			posting("users:3:wallet", "merchants:1:main", "EUR/2", "999999"),
		}}}
		for name, c := range map[string]caller{"AMQP": s.amqp, "NATS": s.nats} {
			err := s.call(c, "v1.createTransaction", env, "declined-"+name, nil)
			var rpcErr *rpc.Error
			if !errors.As(err, &rpcErr) || rpcErr.Code != "REJECTED" || rpcErr.Reason != "exceeds-credits" ||
				rpcErr.Posting == nil || *rpcErr.Posting != 1 || rpcErr.RequestID == "" {
				t.Fatalf("%s decline = %v", name, err)
			}
		}
	})

	t.Run("a write without a key is refused", func(t *testing.T) {
		err := s.call(s.nats, "v1.createTransaction", write, "", nil)
		var rpcErr *rpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.Reason != "missing-idempotency-key" {
			t.Fatalf("no key = %v", err)
		}
	})

	t.Run("a call without a credential is refused; liveness is open", func(t *testing.T) {
		for name, c := range map[string]caller{"AMQP": s.amqp, "NATS": s.nats} {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			answer, err := c.Call(ctx, "v1.listLedgers", rpc.Envelope{}, rpc.Headers{})
			cancel()
			var rpcErr *rpc.Error
			if err == nil {
				err = rpc.DecodeReply(answer.Body, nil)
			}
			if !errors.As(err, &rpcErr) || rpcErr.Code != "UNAUTHENTICATED" {
				t.Fatalf("%s without a credential = %v", name, err)
			}
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			answer, err = c.Call(ctx, "v1.getLiveness", rpc.Envelope{}, rpc.Headers{})
			cancel()
			if err == nil {
				err = rpc.DecodeReply(answer.Body, nil)
			}
			if err != nil {
				t.Fatalf("%s liveness without a credential = %v", name, err)
			}
		}
	})

	t.Run("readiness reports both brokers", func(t *testing.T) {
		resp, err := http.Get(s.rest.URL + "/readyz")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body struct {
			Status       string
			Dependencies map[string]struct{ Status string }
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if resp.StatusCode != http.StatusOK || body.Status != "ready" ||
			body.Dependencies["rabbitmq"].Status != "up" || body.Dependencies["nats"].Status != "up" {
			t.Fatalf("readiness = %d %+v", resp.StatusCode, body)
		}
	})
}
