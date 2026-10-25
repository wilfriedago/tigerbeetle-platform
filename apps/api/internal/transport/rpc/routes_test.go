package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	ledgersapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	platformrpc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// The fakes embed the use case interfaces, so each implements only what a test calls.

type fakeLedgerQueries struct{ ledgersapp.LedgerQueries }

func (fakeLedgerQueries) Get(_ context.Context, name string) (ledgersapp.LedgerView, error) {
	return ledgersapp.LedgerView{}, fmt.Errorf("postgres: get %s: secret driver text: %w", name, ledger.ErrNotFound)
}

type fakeTransactions struct {
	bookkeepingapp.TransactionCommands
	got bookkeepingapp.CreateTransactionInput
}

func (f *fakeTransactions) Create(_ context.Context, in bookkeepingapp.CreateTransactionInput) (bookkeepingapp.TransactionView, error) {
	f.got = in
	id, _ := ledger.ParseID("0192f3a1b7c94e2a8d5f6c0b1e2d7c2e")
	return bookkeepingapp.TransactionView{ID: id, Ledger: in.Ledger, IdempotencyKey: in.Key, RequestID: in.RequestID, Metadata: in.Metadata}, nil
}

type fakeHolds struct {
	bookkeepingapp.HoldQueries
	got bookkeepingapp.ListHoldsInput
}

func (f *fakeHolds) List(_ context.Context, in bookkeepingapp.ListHoldsInput) ([]bookkeepingapp.HoldView, string, domain.HoldCounts, error) {
	f.got = in
	return nil, "next-page", domain.HoldCounts{Open: 1}, nil
}

type fixture struct {
	dispatcher   *platformrpc.Dispatcher
	transactions *fakeTransactions
	holds        *fakeHolds
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{transactions: &fakeTransactions{}, holds: &fakeHolds{}}
	router := Router(Modules{
		Ledgers:     ledgersapp.Application{LedgerQueries: fakeLedgerQueries{}},
		Bookkeeping: bookkeepingapp.Application{TransactionCommands: f.transactions, HoldQueries: f.holds},
	})
	f.dispatcher = platformrpc.NewDispatcher(router, 30*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return f
}

func (f *fixture) call(t *testing.T, c platformrpc.Call) map[string]any {
	t.Helper()
	answer := f.dispatcher.Answer(t.Context(), c)
	if strings.Contains(string(answer.Body), "secret") {
		t.Fatalf("reply leaks: %s", answer.Body)
	}
	var reply map[string]any
	if err := json.Unmarshal(answer.Body, &reply); err != nil {
		t.Fatal(err)
	}
	return reply
}

func TestCreateTransaction(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	reply := f.call(t, platformrpc.Call{
		Method:    "v1.createTransaction",
		Payload:   []byte(`{"params":{"ledger":"eu"},"body":{"postings":[{"source":"world","destination":"users:1:wallet","asset":"EUR/2","amount":"100"}],"metadata":{"order":"7"}}}`),
		RequestID: "req-1", IdempotencyKey: "key-1",
	})
	if reply["ok"] != true {
		t.Fatalf("reply = %v", reply)
	}
	in := f.transactions.got
	if in.Ledger != "eu" || in.Key != "key-1" || in.RequestID != "req-1" || len(in.Postings) != 1 || in.Metadata["order"] != "7" {
		t.Fatalf("use case input = %+v", in)
	}
	data := reply["data"].(map[string]any)
	if data["id"] != "0192f3a1b7c94e2a8d5f6c0b1e2d7c2e" || data["idempotencyKey"] != "key-1" {
		t.Errorf("data = %v", data)
	}

	bad := newFixture(t)
	reply = bad.call(t, platformrpc.Call{
		Method:  "v1.createTransaction",
		Payload: []byte(`{"params":{"ledger":"eu"},"body":{"postings":[{"source":"world","destination":"users:1:wallet","asset":"EUR/2","amount":"x"}]}}`),
	})
	e := reply["error"].(map[string]any)
	if e["code"] != "INVALID" || e["posting"] != float64(0) || bad.transactions.got.Key != "" {
		t.Fatalf("invalid posting: %v, use case ran with %+v", reply, bad.transactions.got)
	}
}

func TestListHoldsParams(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	reply := f.call(t, platformrpc.Call{
		Method:  "v1.listHolds",
		Payload: []byte(`{"params":{"ledger":"eu","status":"open","address":"users:","cursor":"c1","limit":10}}`),
	})
	want := bookkeepingapp.ListHoldsInput{Ledger: "eu", Status: "open", Address: "users:", Cursor: "c1", Limit: 10}
	if diff := cmp.Diff(want, f.holds.got); diff != "" {
		t.Errorf("use case input (-want +got):\n%s", diff)
	}
	data := reply["data"].(map[string]any)
	if data["next"] != "next-page" || data["counts"].(map[string]any)["open"] != float64(1) {
		t.Errorf("data = %v", data)
	}
}

func TestNotFoundKeepsDriverTextOut(t *testing.T) {
	t.Parallel()
	reply := newFixture(t).call(t, platformrpc.Call{Method: "v1.getLedger", Payload: []byte(`{"params":{"ledger":"nope"}}`)})
	e := reply["error"].(map[string]any)
	if reply["ok"] != false || e["code"] != "NOT_FOUND" || e["status"] != float64(404) {
		t.Fatalf("reply = %v", reply)
	}
}

func TestLiveness(t *testing.T) {
	t.Parallel()
	reply := newFixture(t).call(t, platformrpc.Call{Method: "v1.getLiveness"})
	if reply["ok"] != true || reply["data"].(map[string]any)["status"] != "ok" {
		t.Fatalf("reply = %v", reply)
	}
}
