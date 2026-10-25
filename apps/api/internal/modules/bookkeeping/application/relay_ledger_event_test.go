package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

type relayAccounts struct {
	domain.AccountRepository
	rows []domain.AccountRow
	err  error
}

func (f relayAccounts) AccountsByID(_ context.Context, _ []ledger.ID) ([]domain.AccountRow, error) {
	return f.rows, f.err
}

type recordingPublisher struct {
	got []domain.LedgerEvent
	err error
}

func (p *recordingPublisher) Publish(_ context.Context, events ...domain.LedgerEvent) error {
	if p.err != nil {
		return p.err
	}
	p.got = append(p.got, events...)
	return nil
}

func TestLedgerEventRelay(t *testing.T) {
	t.Parallel()
	world, wallet, foreign := ledger.ID{1}, ledger.ID{2}, ledger.ID{9}
	rows := []domain.AccountRow{
		{ID: world, Ledger: "eu", Address: "world", Asset: "EUR/2", TBLedger: 1},
		{ID: wallet, Ledger: "eu", Address: "users:1:wallet", Asset: "EUR/2", TBLedger: 1},
	}
	ours := domain.ChangeEvent{Type: domain.ChangeSinglePhase, Timestamp: 10, TBLedger: 1, Debit: domain.ChangeAccount{ID: world}, Credit: domain.ChangeAccount{ID: wallet}}
	theirs := domain.ChangeEvent{Type: domain.ChangeSinglePhase, Timestamp: 11, TBLedger: 7, Debit: domain.ChangeAccount{ID: foreign}, Credit: domain.ChangeAccount{ID: foreign}}

	pub := &recordingPublisher{}
	if err := NewLedgerEventRelay(relayAccounts{rows: rows}, pub).Relay(t.Context(), ours, theirs); err != nil {
		t.Fatal(err)
	}
	want := []domain.LedgerEvent{
		{ChangeEvent: ours, Ledger: "eu", Asset: "EUR/2", DebitAddress: "world", CreditAddress: "users:1:wallet"},
		{ChangeEvent: theirs},
	}
	if diff := cmp.Diff(want, pub.got, cmpopts.EquateComparable(ledger.Amount{})); diff != "" {
		t.Errorf("published (-want +got):\n%s", diff)
	}

	for name, relay := range map[string]*LedgerEventRelay{
		"postgres down": NewLedgerEventRelay(relayAccounts{err: ledger.ErrUnavailable}, &recordingPublisher{}),
		"kafka down":    NewLedgerEventRelay(relayAccounts{rows: rows}, &recordingPublisher{err: ledger.ErrUnavailable}),
	} {
		if err := relay.Relay(t.Context(), ours); !errors.Is(err, ledger.ErrUnavailable) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
