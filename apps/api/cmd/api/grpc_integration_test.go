//go:build integration

package main

import (
	"context"
	"net"
	"net/http"
	"slices"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

// grpcConn serves the app's gRPC server on an in-memory listener and dials it.
func (h *harness) grpcConn() *grpc.ClientConn {
	h.t.Helper()
	srv := h.app.GRPC()
	if srv == nil {
		h.t.Fatal("gRPC is not enabled in the harness")
	}
	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	h.t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestGRPCEndToEnd runs gRPC through bootstrap.Build against real Postgres and TigerBeetle: the
// same use cases as REST, the same idempotency records, the problems as google.rpc details.
func TestGRPCEndToEnd(t *testing.T) {
	h := newHarness(t)
	conn := h.grpcConn()
	ctx := t.Context()
	ledgers := pb.NewLedgerServiceClient(conn)
	transactions := pb.NewTransactionServiceClient(conn)

	if _, err := ledgers.CreateLedger(ctx, &pb.CreateLedgerRequest{Ledger: "rpc"}); err != nil {
		t.Fatalf("CreateLedger: %v", err)
	}

	posting := &pb.Posting{Source: "world", Destination: "users:1:wallet", Asset: "EUR/2", Amount: "1250"}
	created, err := transactions.CreateTransaction(ctx, &pb.CreateTransactionRequest{
		Ledger: "rpc", IdempotencyKey: "grpc-1", Postings: []*pb.Posting{posting},
	})
	if err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	t.Run("a gRPC write replays over REST and gRPC under its key", func(t *testing.T) {
		replay := h.do(http.MethodPost, "/v1/ledgers/rpc/transactions", "grpc-1",
			map[string]any{"postings": []any{map[string]any{"source": "world", "destination": "users:1:wallet", "asset": "EUR/2", "amount": "1250"}}})
		h.expect(replay, http.StatusCreated, "REST replay of a gRPC write")
		if replay.str("id") != created.GetTransaction().GetId() {
			t.Fatalf("REST replay answered %s, gRPC created %s", replay.str("id"), created.GetTransaction().GetId())
		}
		again, err := transactions.CreateTransaction(ctx, &pb.CreateTransactionRequest{
			Ledger: "rpc", IdempotencyKey: "grpc-1", Postings: []*pb.Posting{posting},
		})
		if err != nil || !proto.Equal(again.GetTransaction(), created.GetTransaction()) {
			t.Fatalf("gRPC replay = %v, %v", again, err)
		}
		_, err = transactions.CreateTransaction(ctx, &pb.CreateTransactionRequest{
			Ledger: "rpc", IdempotencyKey: "grpc-1",
			Postings: []*pb.Posting{{Source: "world", Destination: "users:1:wallet", Asset: "EUR/2", Amount: "1"}},
		})
		if info := errorInfo(t, err, codes.FailedPrecondition); info.GetReason() != "IDEMPOTENCY_KEY_REUSED" {
			t.Fatalf("reused key: %v", info)
		}
	})

	t.Run("reads", func(t *testing.T) {
		account, err := pb.NewAccountServiceClient(conn).GetAccount(ctx, &pb.GetAccountRequest{Ledger: "rpc", Address: "users:1:wallet"})
		if err != nil || account.GetAccount().GetBalances()[0].GetBalance() != "1250" {
			t.Fatalf("GetAccount = %v, %v", account, err)
		}
		list, err := transactions.ListTransactions(ctx, &pb.ListTransactionsRequest{Ledger: "rpc", PageSize: 10})
		if err != nil || len(list.GetTransactions()) != 1 || list.GetNextPageToken() != "" {
			t.Fatalf("ListTransactions = %v, %v", list, err)
		}
		_, err = pb.NewHoldServiceClient(conn).GetHold(ctx, &pb.GetHoldRequest{Ledger: "rpc", Id: "00000000000000000000000000000001"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("GetHold of a missing hold = %v", err)
		}
	})

	t.Run("a declined chain carries ErrorInfo", func(t *testing.T) {
		_, err := transactions.CreateTransaction(ctx, &pb.CreateTransactionRequest{
			Ledger: "rpc", IdempotencyKey: "grpc-declined",
			Postings: []*pb.Posting{
				{Source: "world", Destination: "users:2:wallet", Asset: "EUR/2", Amount: "5"},
				{Source: "users:2:wallet", Destination: "merchants:1:main", Asset: "EUR/2", Amount: "999999"},
			},
		})
		info := errorInfo(t, err, codes.FailedPrecondition)
		if info.GetReason() != "EXCEEDS_CREDITS" || info.GetMetadata()["posting"] != "1" || info.GetMetadata()["type"] != problem.TypeOf("exceeds-credits") {
			t.Fatalf("decline: %v", info)
		}
	})

	t.Run("a write in flight answers with RetryInfo", func(t *testing.T) {
		h.faults.dropBefore.Store(true)
		req := &pb.CreateTransactionRequest{
			Ledger: "rpc", IdempotencyKey: "grpc-lost",
			Postings: []*pb.Posting{{Source: "world", Destination: "users:3:wallet", Asset: "EUR/2", Amount: "700"}},
		}
		_, err := transactions.CreateTransaction(ctx, req)
		if info := errorInfo(t, err, codes.Unavailable); info.GetReason() != "WRITE_PENDING" || retryDelay(err) <= 0 {
			t.Fatalf("lost request: %v, retry %v", info, retryDelay(err))
		}
		_, err = transactions.CreateTransaction(ctx, req)
		if info := errorInfo(t, err, codes.Aborted); info.GetReason() != "REQUEST_IN_FLIGHT" || retryDelay(err) <= 0 {
			t.Fatalf("retry while pending: %v, retry %v", info, retryDelay(err))
		}
		if n := h.app.ReconcileOnce(ctx); n != 1 {
			t.Fatalf("reconciler settled %d records, want 1", n)
		}
		if _, err := transactions.CreateTransaction(ctx, req); err != nil {
			t.Fatalf("outcome after settling: %v", err)
		}
	})

	t.Run("health and reflection", func(t *testing.T) {
		resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
		if err != nil || resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			t.Fatalf("health = %v, %v", resp, err)
		}
		stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := stream.Send(&reflectionpb.ServerReflectionRequest{MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{}}); err != nil {
			t.Fatal(err)
		}
		got, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, s := range got.GetListServicesResponse().GetService() {
			names = append(names, s.GetName())
		}
		for _, want := range []string{"tigerbeetle.platform.v1.TransactionService", "grpc.health.v1.Health"} {
			if !slices.Contains(names, want) {
				t.Errorf("reflection lists %v, missing %s", names, want)
			}
		}
	})
}

func errorInfo(t *testing.T, err error, code codes.Code) *errdetails.ErrorInfo {
	t.Helper()
	st := status.Convert(err)
	if st.Code() != code {
		t.Fatalf("code = %v (%s), want %v", st.Code(), st.Message(), code)
	}
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info
		}
	}
	t.Fatalf("no ErrorInfo in %v", st.Proto())
	return nil
}

func retryDelay(err error) float64 {
	for _, d := range status.Convert(err).Details() {
		if r, ok := d.(*errdetails.RetryInfo); ok {
			return r.GetRetryDelay().AsDuration().Seconds()
		}
	}
	return 0
}
