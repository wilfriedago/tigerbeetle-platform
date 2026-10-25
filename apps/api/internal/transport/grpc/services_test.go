package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	ledgersapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcserver"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/readiness"
)

var (
	created = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	txID    = mustID("0192f3a1b7c94e2a8d5f6c0b1e2d7c2e")
	legID   = mustID("0192f3a1b7c94e2a8d5f6c0b1e2d7c2f")
)

// The fakes embed the use case interfaces, so each implements only the methods a test calls; any
// other call panics, which the recovery interceptor turns into Internal.

type fakeLedgers struct {
	ledgersapp.LedgerCommands
	createdName string
}

func (f *fakeLedgers) Create(_ context.Context, name string, md map[string]string) (ledgersapp.LedgerView, error) {
	f.createdName = name
	return ledgersapp.LedgerView{
		Ledger:    ledger.Ledger{Name: name, Metadata: md, CreatedAt: created},
		Footprint: ledger.Footprint{Assets: []string{"EUR/2"}, Accounts: 3},
	}, nil
}

type fakeLedgerQueries struct{ ledgersapp.LedgerQueries }

func (fakeLedgerQueries) Get(_ context.Context, name string) (ledgersapp.LedgerView, error) {
	return ledgersapp.LedgerView{}, fmt.Errorf("postgres: get %s: secret driver text: %w", name, ledger.ErrNotFound)
}

type fakeTransactions struct {
	bookkeepingapp.TransactionCommands
	got     bookkeepingapp.CreateTransactionInput
	decline bool
}

func (f *fakeTransactions) Create(_ context.Context, in bookkeepingapp.CreateTransactionInput) (bookkeepingapp.TransactionView, error) {
	f.got = in
	if f.decline {
		return bookkeepingapp.TransactionView{}, &ledger.Problem{
			Kind: ledger.ErrRejected, Reason: "exceeds-credits", Detail: "posting 1 would go below zero",
			Posting: 0, Results: []string{"exceeds_credits"},
		}
	}
	p := in.Postings[0]
	return bookkeepingapp.TransactionView{
		ID: txID, Ledger: in.Ledger, IdempotencyKey: in.Key, RequestID: in.RequestID,
		EventType: ledger.EventType{Name: "default", Code: 1}, Metadata: in.Metadata, CommittedAt: created, Timestamp: 1761393600000000000,
		Postings: []bookkeepingapp.PostingView{{
			Source: p.Source, Destination: p.Destination, Asset: p.Asset.String(), Amount: p.Amount, TransferID: legID, TBLedger: 1,
		}},
	}, nil
}

type fakeHolds struct {
	bookkeepingapp.HoldQueries
	got bookkeepingapp.ListHoldsInput
}

func (f *fakeHolds) List(_ context.Context, in bookkeepingapp.ListHoldsInput) ([]bookkeepingapp.HoldView, string, domain.HoldCounts, error) {
	f.got = in
	return nil, "next-page", domain.HoldCounts{Open: 2}, nil
}

func mustID(s string) ledger.ID {
	id, err := ledger.ParseID(s)
	if err != nil {
		panic(err)
	}
	return id
}

type harness struct {
	conn         *grpclib.ClientConn
	ledgers      *fakeLedgers
	transactions *fakeTransactions
	holds        *fakeHolds
}

func newHarness(t *testing.T, tigerbeetleUp bool) *harness {
	t.Helper()
	h := &harness{ledgers: &fakeLedgers{}, transactions: &fakeTransactions{}, holds: &fakeHolds{}}
	checker := readiness.New(time.Second)
	checker.Require(healthdto.ProbePostgres, func(context.Context) error { return nil })
	checker.Require(healthdto.ProbeTigerBeetle, func(context.Context) error {
		if tigerbeetleUp {
			return nil
		}
		return errors.New("secret driver text")
	})
	srv := grpcserver.New(grpcserver.Options{ShutdownTimeout: time.Second, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	RegisterAll(srv, Modules{
		Health:      healthdto.Source{Readiness: checker, Service: healthdto.Service{Name: "tigerbeetle-platform-api", Version: "test"}},
		Ledgers:     ledgersapp.Application{LedgerCommands: h.ledgers, LedgerQueries: fakeLedgerQueries{}},
		Bookkeeping: bookkeepingapp.Application{TransactionCommands: h.transactions, HoldQueries: h.holds},
	})
	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpclib.NewClient("passthrough:///bufconn",
		grpclib.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpclib.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	h.conn = conn
	return h
}

func TestLedgerService(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true)
	client := pb.NewLedgerServiceClient(h.conn)

	resp, err := client.CreateLedger(t.Context(), &pb.CreateLedgerRequest{Ledger: "eu", Metadata: map[string]string{"team": "payments"}})
	if err != nil {
		t.Fatal(err)
	}
	want := &pb.Ledger{Name: "eu", Metadata: map[string]string{"team": "payments"}, Assets: []string{"EUR/2"}, Accounts: 3}
	got := resp.GetLedger()
	if !got.GetCreatedAt().AsTime().Equal(created) {
		t.Errorf("created_at = %v", got.GetCreatedAt().AsTime())
	}
	got.CreatedAt = nil
	if !proto.Equal(want, got) {
		t.Errorf("ledger = %v, want %v", got, want)
	}

	_, err = client.GetLedger(t.Context(), &pb.GetLedgerRequest{Ledger: "nope"})
	st := status.Convert(err)
	if st.Code() != codes.NotFound || strings.Contains(st.Proto().String(), "secret") {
		t.Fatalf("GetLedger of a missing ledger = %v", st.Proto())
	}
}

func TestTransactionService(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true)
	client := pb.NewTransactionServiceClient(h.conn)
	posting := &pb.Posting{Source: "world", Destination: "users:1:wallet", Asset: "EUR/2", Amount: "1250"}

	ctx := metadata.AppendToOutgoingContext(t.Context(), grpcserver.RequestIDKey, "req-42")
	resp, err := client.CreateTransaction(ctx, &pb.CreateTransactionRequest{
		Ledger: "eu", IdempotencyKey: "key-1", Postings: []*pb.Posting{posting}, Metadata: map[string]string{"order": "7"},
	})
	if err != nil {
		t.Fatal(err)
	}
	in := h.transactions.got
	if in.Ledger != "eu" || in.Key != "key-1" || in.RequestID != "req-42" || len(in.Postings) != 1 || in.Postings[0].Amount.String() != "1250" {
		t.Fatalf("use case input = %+v", in)
	}
	tx := resp.GetTransaction()
	if tx.GetId() != txID.String() || tx.GetPostings()[0].GetAmount() != "1250" || tx.GetPostings()[0].GetTransferId() != legID.String() ||
		tx.GetTimestamp() != "1761393600000000000" || tx.GetIdempotencyKey() != "key-1" || tx.GetRequestId() != "req-42" {
		t.Fatalf("transaction = %v", tx)
	}

	t.Run("an invalid posting names its field and calls nothing", func(t *testing.T) {
		h := newHarness(t, true)
		bad := &pb.Posting{Source: "world", Destination: "users:1:wallet", Asset: "EUR/2", Amount: "-1"}
		_, err := pb.NewTransactionServiceClient(h.conn).CreateTransaction(t.Context(), &pb.CreateTransactionRequest{
			Ledger: "eu", IdempotencyKey: "key-2", Postings: []*pb.Posting{posting, bad},
		})
		st := status.Convert(err)
		bad2 := findDetail[*errdetails.BadRequest](st)
		if st.Code() != codes.InvalidArgument || bad2 == nil || bad2.GetFieldViolations()[0].GetField() != "postings[1]" {
			t.Fatalf("status = %v", st.Proto())
		}
		if h.transactions.got.Key != "" {
			t.Errorf("the use case ran with %+v", h.transactions.got)
		}
	})

	t.Run("a declined chain carries the problem members", func(t *testing.T) {
		h := newHarness(t, true)
		h.transactions.decline = true
		_, err := pb.NewTransactionServiceClient(h.conn).CreateTransaction(t.Context(), &pb.CreateTransactionRequest{
			Ledger: "eu", IdempotencyKey: "key-3", Postings: []*pb.Posting{posting},
		})
		st := status.Convert(err)
		info := findDetail[*errdetails.ErrorInfo](st)
		if st.Code() != codes.FailedPrecondition || info.GetReason() != "EXCEEDS_CREDITS" ||
			info.GetMetadata()["posting"] != "0" || info.GetMetadata()["results"] != "exceeds_credits" {
			t.Fatalf("status = %v", st.Proto())
		}
	})
}

func TestHoldServiceFilters(t *testing.T) {
	t.Parallel()
	h := newHarness(t, true)
	resp, err := pb.NewHoldServiceClient(h.conn).ListHolds(t.Context(), &pb.ListHoldsRequest{
		Ledger: "eu", Status: pb.HoldStatus_HOLD_STATUS_OPEN, PageSize: 10, PageToken: "cursor-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := bookkeepingapp.ListHoldsInput{Ledger: "eu", Status: "open", Cursor: "cursor-1", Limit: 10}
	if diff := cmp.Diff(want, h.holds.got); diff != "" {
		t.Errorf("use case input (-want +got):\n%s", diff)
	}
	if resp.GetNextPageToken() != "next-page" || resp.GetCounts().GetOpen() != 2 {
		t.Errorf("response = %v", resp)
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()
	for _, up := range []bool{true, false} {
		h := newHarness(t, up)
		resp, err := healthpb.NewHealthClient(h.conn).Check(t.Context(), &healthpb.HealthCheckRequest{})
		if err != nil {
			t.Fatal(err)
		}
		want := healthpb.HealthCheckResponse_NOT_SERVING
		if up {
			want = healthpb.HealthCheckResponse_SERVING
		}
		if resp.GetStatus() != want {
			t.Errorf("TigerBeetle up=%v: status %v, want %v", up, resp.GetStatus(), want)
		}
	}

	h := newHarness(t, false)
	info, err := pb.NewOperationsServiceClient(h.conn).GetInfo(t.Context(), &pb.GetInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	tb := info.GetDependencies()["tigerbeetle"]
	if info.GetService() != grpcserver.ErrorDomain || tb.GetStatus() != pb.DependencyStatus_DEPENDENCY_STATUS_DOWN || tb.GetError() != "unreachable" {
		t.Fatalf("info = %v", info)
	}
	if info.GetDependencies()["postgres"].GetStatus() != pb.DependencyStatus_DEPENDENCY_STATUS_UP {
		t.Errorf("postgres = %v", info.GetDependencies()["postgres"])
	}
}

func findDetail[T proto.Message](st *status.Status) T {
	var zero T
	for _, d := range st.Details() {
		if v, ok := d.(T); ok {
			return v
		}
	}
	return zero
}
