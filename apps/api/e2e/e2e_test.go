//go:build e2e

// Package e2e calls the compose stack from the host through its published ports, on every
// transport: REST, GraphQL, gRPC, AMQP RPC and NATS RPC (plan section 11.4). Start the stack with
// `just compose-up-brokers`, then run `just test-e2e`. The addresses default to the compose
// ports; E2E_API_URL, E2E_GRPC_ADDR, E2E_AMQP_URL and E2E_NATS_URL override them.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/amqprpc"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/natsrpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// caller is what the AMQP and NATS clients have in common.
type caller interface {
	Call(ctx context.Context, method string, env rpc.Envelope, key, requestID string) (rpc.Answer, error)
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

var (
	apiURL   = env("E2E_API_URL", "http://localhost:8080")
	grpcAddr = env("E2E_GRPC_ADDR", "localhost:9090")
	amqpURL  = env("E2E_AMQP_URL", "amqp://guest:guest@localhost:5672/")
	natsURL  = env("E2E_NATS_URL", "nats://localhost:4222")
)

func postJSON(t *testing.T, path, key string, body, out any) int {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, apiURL+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("POST %s: decode: %v", path, err)
		}
	}
	return resp.StatusCode
}

// TestEveryTransport writes one transaction over GraphQL and replays it under its key over REST,
// gRPC, AMQP and NATS: every transport reaches the same use case and the same record.
func TestEveryTransport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ledger := "e2e-" + requestid.New()[:12]
	key := "e2e-" + requestid.New()
	postings := []any{map[string]string{"source": "world", "destination": "users:1:wallet", "asset": "EUR/2", "amount": "1250"}}

	if code := postJSON(t, "/v1/ledgers/"+ledger, "", map[string]any{}, nil); code != http.StatusCreated {
		t.Fatalf("REST createLedger = %d", code)
	}

	var gql struct {
		Data struct {
			CreateTransaction struct{ ID string } `json:"createTransaction"`
		}
		Errors []any
	}
	postJSON(t, "/graphql", "", map[string]any{
		"query":     `mutation($input: CreateTransactionInput!) { createTransaction(input: $input) { id } }`,
		"variables": map[string]any{"input": map[string]any{"ledger": ledger, "idempotencyKey": key, "postings": postings}},
	}, &gql)
	id := gql.Data.CreateTransaction.ID
	if id == "" || gql.Errors != nil {
		t.Fatalf("GraphQL createTransaction = %+v", gql)
	}

	t.Run("REST", func(t *testing.T) {
		var tx struct{ ID string }
		if code := postJSON(t, "/v1/ledgers/"+ledger+"/transactions", key, map[string]any{"postings": postings}, &tx); code != http.StatusCreated || tx.ID != id {
			t.Fatalf("REST replay = %d %s, want %s", code, tx.ID, id)
		}
	})

	t.Run("gRPC", func(t *testing.T) {
		conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close() }()
		health, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
		if err != nil || health.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			t.Fatalf("health = %v, %v", health, err)
		}
		resp, err := pb.NewTransactionServiceClient(conn).CreateTransaction(ctx, &pb.CreateTransactionRequest{
			Ledger: ledger, IdempotencyKey: key,
			Postings: []*pb.Posting{{Source: "world", Destination: "users:1:wallet", Asset: "EUR/2", Amount: "1250"}},
		})
		if err != nil || resp.GetTransaction().GetId() != id {
			t.Fatalf("gRPC replay = %v, %v; want %s", resp, err, id)
		}
	})

	amqpClient, err := amqprpc.Dial(amqpURL, "tbp.rpc.v1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = amqpClient.Close() }()
	natsClient, err := natsrpc.Dial(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer natsClient.Close()
	for name, c := range map[string]caller{"AMQP": amqpClient, "NATS": natsClient} {
		t.Run(name, func(t *testing.T) {
			answer, err := c.Call(ctx, "v1.createTransaction", rpc.Envelope{
				Params: map[string]string{"ledger": ledger}, Body: map[string]any{"postings": postings},
			}, key, "")
			if err != nil {
				t.Fatal(err)
			}
			var tx struct{ ID string }
			if err := rpc.DecodeReply(answer.Body, &tx); err != nil || tx.ID != id {
				t.Fatalf("%s replay = %+v, %v; want %s", name, tx, err, id)
			}
		})
	}

	t.Run("one balance and every broker up", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/v1/ledgers/%s/accounts/users:1:wallet", apiURL, ledger))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var account struct {
			Balances []struct{ Balance string }
		}
		_ = json.NewDecoder(resp.Body).Decode(&account)
		if len(account.Balances) != 1 || account.Balances[0].Balance != "1250" {
			t.Fatalf("balance after five sends of one write = %+v, want 1250", account.Balances)
		}
		ready, err := http.Get(apiURL + "/readyz")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ready.Body.Close() }()
		var body struct {
			Status       string
			Dependencies map[string]struct{ Status string }
		}
		_ = json.NewDecoder(ready.Body).Decode(&body)
		if body.Status != "ready" || body.Dependencies["rabbitmq"].Status != "up" || body.Dependencies["nats"].Status != "up" {
			t.Fatalf("readiness = %+v", body)
		}
	})
}
