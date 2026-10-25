package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// LedgerEventRelay turns TigerBeetle change events into ledger events (ADR-0022): it adds the
// ledger, asset and addresses Postgres recorded before the transfer existed, then publishes.
type LedgerEventRelay struct {
	accounts  domain.AccountRepository
	publisher domain.LedgerEventPublisher
}

// NewLedgerEventRelay builds the relay.
func NewLedgerEventRelay(accounts domain.AccountRepository, publisher domain.LedgerEventPublisher) *LedgerEventRelay {
	return &LedgerEventRelay{accounts: accounts, publisher: publisher}
}

// Relay enriches and publishes events, in order. It returns once the publisher has them; an error
// means none may be taken as published, and the caller retries them all.
func (r *LedgerEventRelay) Relay(ctx context.Context, events ...domain.ChangeEvent) error {
	if len(events) == 0 {
		return nil
	}
	ids := make([]ledger.ID, 0, 2*len(events))
	for i := range events {
		ids = append(ids, events[i].Debit.ID, events[i].Credit.ID)
	}
	rows, err := r.accounts.AccountsByID(ctx, ids)
	if err != nil {
		return fmt.Errorf("relay: find accounts: %w", err)
	}
	known := make(map[ledger.ID]domain.AccountRow, len(rows))
	for i := range rows {
		known[rows[i].ID] = rows[i]
	}
	out := make([]domain.LedgerEvent, len(events))
	for i := range events {
		ev := domain.LedgerEvent{ChangeEvent: events[i]}
		debit, hasDebit := known[ev.Debit.ID]
		credit, hasCredit := known[ev.Credit.ID]
		if hasDebit {
			ev.DebitAddress = string(debit.Address)
			ev.Ledger, ev.Asset = debit.Ledger, debit.Asset
		}
		if hasCredit {
			ev.CreditAddress = string(credit.Address)
			// Both accounts share the TigerBeetle ledger, so either names the ledger and asset.
			ev.Ledger, ev.Asset = credit.Ledger, credit.Asset
		}
		out[i] = ev
	}
	if err := r.publisher.Publish(ctx, out...); err != nil {
		return fmt.Errorf("relay: publish: %w", err)
	}
	return nil
}
