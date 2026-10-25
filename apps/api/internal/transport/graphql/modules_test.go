package graphql

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	ledgersapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
)

// fakeLedgers answers the ledgers use cases the tests reach; the embedded interfaces are nil, so a
// call the test does not expect panics and fails it.
type fakeLedgers struct {
	ledgersapp.LedgerQueries
	ledgersapp.SettingsQueries
}

func (fakeLedgers) Get(_ context.Context, name string) (ledgersapp.LedgerView, error) {
	if name != "eu" {
		return ledgersapp.LedgerView{}, fmt.Errorf("get ledger %q: %w", name, ledger.ErrNotFound)
	}
	return ledgersapp.LedgerView{
		Ledger:    ledger.Ledger{Name: "eu", Metadata: map[string]string{"region": "eu-west"}, CreatedAt: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)},
		Footprint: ledger.Footprint{Assets: []string{"EUR/2"}, Accounts: 3},
	}, nil
}

func (fakeLedgers) Rules(context.Context, string) ([]ledger.Rule, error) {
	return ledger.EffectiveRules(nil), nil
}

// fakeBookkeeping answers the bookkeeping use cases the tests reach.
type fakeBookkeeping struct {
	bookkeepingapp.OverviewQueries
	bookkeepingapp.TransactionCommands
	err error
}

func (f fakeBookkeeping) Stats(context.Context, string) (bookkeepingapp.Stats, error) {
	return bookkeepingapp.Stats{Accounts: 3, AccountsToday: 1, Holds: domain.HoldCounts{Open: 2}}, f.err
}

func (f fakeBookkeeping) Create(context.Context, bookkeepingapp.CreateTransactionInput) (bookkeepingapp.TransactionView, error) {
	return bookkeepingapp.TransactionView{}, f.err
}

func modulesWith(bk fakeBookkeeping) Modules {
	m := healthModules()
	m.Ledgers = ledgersapp.Application{LedgerQueries: fakeLedgers{}, SettingsQueries: fakeLedgers{}}
	m.Bookkeeping = bookkeepingapp.Application{OverviewQueries: bk, TransactionCommands: bk}
	return m
}

func TestLedgerAcrossModules(t *testing.T) {
	t.Parallel()
	out := serve(t, modulesWith(fakeBookkeeping{}), `{"query":"{ ledger(name: \"eu\") { name metadata assetCodes accountCount rules { pattern builtIn } stats { accounts accountsToday holds { open } } } missing: ledger(name: \"nope\") { name } }"}`)
	if out["errors"] != nil {
		t.Fatalf("errors = %v", out["errors"])
	}
	want := map[string]any{
		"ledger": map[string]any{
			"name": "eu", "metadata": map[string]any{"region": "eu-west"}, "assetCodes": []any{"EUR/2"}, "accountCount": float64(3),
			"rules": []any{
				map[string]any{"pattern": "world", "builtIn": true},
				map[string]any{"pattern": "*", "builtIn": true},
			},
			"stats": map[string]any{"accounts": float64(3), "accountsToday": float64(1), "holds": map[string]any{"open": float64(2)}},
		},
		"missing": nil,
	}
	if diff := cmp.Diff(want, out["data"]); diff != "" {
		t.Fatalf("data (-want +got):\n%s", diff)
	}
}

const createTransaction = `{"query":"mutation($input: CreateTransactionInput!) { createTransaction(input: $input) { id } }",` +
	`"variables":{"input":{"ledger":"eu","idempotencyKey":"k1","postings":[{"source":"world","destination":"users:1","asset":"EUR/2","amount":%s}]}}}`

func TestMutationErrors(t *testing.T) {
	t.Parallel()
	declined := &ledger.Problem{Kind: ledger.ErrRejected, Reason: "exceeds-credits", Detail: "declined", Posting: 0, Results: []string{"exceeds_credits"}}
	tests := []struct {
		name   string
		err    error
		amount string
		want   map[string]any
	}{
		{
			name: "a decline carries its posting and results", err: declined, amount: `"5"`,
			want: map[string]any{"code": "REJECTED", "reason": "exceeds-credits", "status": float64(422), "posting": float64(0), "results": []any{"exceeds_credits"}},
		},
		{
			name: "an outage carries no driver text", err: fmt.Errorf("tigerbeetle: %w: secret driver text", ledger.ErrUnavailable), amount: `"5"`,
			want: map[string]any{"code": "UNAVAILABLE", "reason": "dependency-unavailable", "status": float64(503)},
		},
		{
			name: "an amount sent as a number is invalid", amount: `5`,
			want: map[string]any{"code": "INVALID", "status": float64(400)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := serve(t, modulesWith(fakeBookkeeping{err: tt.err}), fmt.Sprintf(createTransaction, tt.amount))
			errs, _ := out["errors"].([]any)
			if len(errs) != 1 {
				t.Fatalf("errors = %v", out)
			}
			ext := errs[0].(map[string]any)["extensions"].(map[string]any)
			for k, v := range tt.want {
				if diff := cmp.Diff(v, ext[k]); diff != "" {
					t.Errorf("extension %s (-want +got):\n%s", k, diff)
				}
			}
			if ext["requestId"] != "req-1" {
				t.Errorf("requestId = %v", ext["requestId"])
			}
			if strings.Contains(fmt.Sprint(out), "secret") {
				t.Errorf("driver text leaked: %v", out)
			}
		})
	}
}
