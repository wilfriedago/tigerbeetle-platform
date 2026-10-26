package application

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

type world struct {
	app   Application
	store *fakeStore
	tb    *fakeTigerBeetle
	now   time.Time
}

func newWorld() *world {
	w := &world{store: newFakeStore(), tb: newFakeTigerBeetle(), now: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)}
	w.app = New(Deps{
		Accounts: w.store, Writes: w.store, Transactions: w.store, Holds: w.store,
		Ledger: w.tb, IDs: &counterIDs{}, Clock: ledger.ClockFunc(func() time.Time { return w.now }),
		Ledgers: fakeLedgers{}, Policies: fakeLedgers{}, EventTypes: fakeLedgers{},
		GracePeriod: time.Second, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Authorizer: allowAll{},
	})
	return w
}

func posting(source, destination ledger.Address, amount uint64) domain.Posting {
	return domain.Posting{Source: source, Destination: destination, Asset: ledger.Asset{Code: "EUR", Scale: 2}, Amount: ledger.AmountFromUint64(amount)}
}

func problemOf(t *testing.T, err error) *ledger.Problem {
	t.Helper()
	var p *ledger.Problem
	if !errors.As(err, &p) {
		t.Fatalf("error %v is not a problem", err)
	}
	return p
}

func TestCreateTransaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newWorld()
	in := CreateTransactionInput{Ledger: "eu", Key: "k1", Postings: []domain.Posting{posting(ledger.World, "users:1:wallet", 1250)}}

	created, err := w.app.TransactionCommands.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(created.Postings) != 1 || created.Postings[0].Amount.String() != "1250" || created.EventType.Name != "default" {
		t.Fatalf("created = %+v", created)
	}
	if len(w.tb.accounts) != 2 {
		t.Fatalf("accounts created in TigerBeetle = %d, want 2 (world and the wallet)", len(w.tb.accounts))
	}

	replay, err := w.app.TransactionCommands.Create(ctx, in)
	if err != nil || replay.ID != created.ID {
		t.Fatalf("replay = %v, %v; want the same transaction", replay.ID, err)
	}

	in.Postings[0].Amount = ledger.AmountFromUint64(1)
	_, err = w.app.TransactionCommands.Create(ctx, in)
	if p := problemOf(t, err); p.Reason != "idempotency-key-reused" {
		t.Fatalf("reused key = %+v", p)
	}
}

func TestDeclinedTransactionIsRecorded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newWorld()
	w.tb.result = func(s domain.TransferSpec) string {
		if s.Amount.String() == "999" {
			return domain.ResultExceedsCredits
		}
		return domain.ResultCreated
	}
	in := CreateTransactionInput{Ledger: "eu", Key: "k2", Postings: []domain.Posting{
		posting(ledger.World, "users:1:wallet", 5),
		posting("users:1:wallet", "merchants:1:main", 999),
	}}
	_, err := w.app.TransactionCommands.Create(ctx, in)
	p := problemOf(t, err)
	if p.Reason != "exceeds-credits" || p.Posting != 1 || len(p.Results) != 2 || !errors.Is(err, ledger.ErrRejected) {
		t.Fatalf("decline = %+v", p)
	}
	if rec := w.store.writes["k2"]; rec.Status != domain.RequestDeclined || rec.ResponseStatus != domain.StatusRejected {
		t.Fatalf("record = %+v", rec)
	}
	_, err = w.app.TransactionCommands.Create(ctx, in)
	if p := problemOf(t, err); p.Reason != "exceeds-credits" {
		t.Fatalf("replay of a decline = %+v", p)
	}
}

func TestOutageLeavesTheWritePendingForTheReconciler(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newWorld()
	w.tb.failNext = true
	in := CreateTransactionInput{Ledger: "eu", Key: "k3", Postings: []domain.Posting{posting(ledger.World, "users:2:wallet", 7)}}
	_, err := w.app.TransactionCommands.Create(ctx, in)
	if p := problemOf(t, err); p.Reason != "write-pending" || p.RetryAfter == 0 {
		t.Fatalf("outage = %+v", p)
	}
	_, err = w.app.TransactionCommands.Create(ctx, in)
	if p := problemOf(t, err); p.Reason != "request-in-flight" {
		t.Fatalf("retry while pending = %+v", p)
	}

	w.now = w.now.Add(time.Minute)
	if n := w.app.Reconciler.RunOnce(ctx); n != 1 {
		t.Fatalf("reconciler settled %d records, want 1", n)
	}
	if rec := w.store.writes["k3"]; rec.Status != domain.RequestCommitted {
		t.Fatalf("record after the reconciler = %+v", rec)
	}
	got, err := w.app.TransactionCommands.Create(ctx, in)
	if err != nil || got.Postings[0].Amount.String() != "7" {
		t.Fatalf("replay after the reconciler = %+v, %v", got, err)
	}
}

func TestWritesValidateBeforeRecording(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newWorld()
	tests := []struct {
		name   string
		in     CreateTransactionInput
		want   error
		reason string
	}{
		{name: "no key", in: CreateTransactionInput{Ledger: "eu"}, want: ledger.ErrInvalid, reason: "missing-idempotency-key"},
		{name: "unknown ledger", in: CreateTransactionInput{Ledger: "nope", Key: "k"}, want: ledger.ErrNotFound},
		{name: "no posting", in: CreateTransactionInput{Ledger: "eu", Key: "k"}, want: ledger.ErrInvalid},
		{name: "unknown event type", in: CreateTransactionInput{
			Ledger: "eu", Key: "k", EventType: "nope",
			Postings: []domain.Posting{posting(ledger.World, "a", 1)},
		}, want: ledger.ErrInvalid, reason: "unknown-event-type"},
	}
	for _, tt := range tests {
		_, err := w.app.TransactionCommands.Create(ctx, tt.in)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
		if tt.reason != "" && problemOf(t, err).Reason != tt.reason {
			t.Errorf("%s: reason = %q, want %q", tt.name, problemOf(t, err).Reason, tt.reason)
		}
	}
	if len(w.store.writes) != 0 {
		t.Fatalf("an invalid write was recorded: %+v", w.store.writes)
	}
}

func TestDeclineProblemNamesThePosting(t *testing.T) {
	t.Parallel()
	postings := []domain.Posting{posting(ledger.World, "users:1:wallet", 1), posting("users:1:wallet", "merchants:42:main", 9)}
	v := domain.Classify([]domain.TransferOutcome{{Result: domain.ResultLinkedEventFailed}, {Result: domain.ResultExceedsCredits}})
	p := declineProblem(v, postings, nil)
	if p.Reason != "exceeds-credits" || p.Posting != 1 || !errors.Is(p, ledger.ErrRejected) {
		t.Fatalf("problem = %+v", p)
	}
	want := "Posting 2 would take users:1:wallet below zero in EUR/2. Nothing was applied: the postings are linked."
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func TestCanonicalRequestIsStable(t *testing.T) {
	t.Parallel()
	a := transactionRequest{Postings: []postingRequest{{Source: "world", Destination: "users:1", Asset: "EUR/2", Amount: "1"}}, Metadata: map[string]string{"b": "2", "a": "1"}}
	b := transactionRequest{Postings: []postingRequest{{Source: "world", Destination: "users:1", Asset: "EUR/2", Amount: "1"}}, Metadata: map[string]string{"a": "1", "b": "2"}}
	_, ha, err := canonicalRequest(a)
	if err != nil {
		t.Fatal(err)
	}
	_, hb, _ := canonicalRequest(b)
	if !bytes.Equal(ha, hb) {
		t.Fatal("equal requests hashed differently")
	}
	b.Postings[0].Amount = "2"
	_, hc, _ := canonicalRequest(b)
	if bytes.Equal(ha, hc) {
		t.Fatal("different amounts hashed equally")
	}
}

func TestWriteCursorRoundTrip(t *testing.T) {
	t.Parallel()
	c := domain.WriteCursor{CreatedAt: time.Date(2026, 10, 24, 14, 32, 8, 123456000, time.UTC), Key: "idem|with|pipes"}
	got, err := decodeWriteCursor(encodeWriteCursor(c))
	if err != nil {
		t.Fatal(err)
	}
	if !got.CreatedAt.Equal(c.CreatedAt) || got.Key != c.Key {
		t.Fatalf("round trip = %+v, want %+v", got, c)
	}
	if _, err := decodeIDCursor(ledger.EncodeCursor("not-an-id")); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatalf("bad ID cursor error = %v", err)
	}
}
