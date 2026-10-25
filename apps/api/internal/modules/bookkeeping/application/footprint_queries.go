package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// FootprintQueries answers the ledgers module's ports: what each ledger holds, how often each event
// type is used, and the assets of an address. They read only bookkeeping's tables.
type FootprintQueries struct {
	accounts domain.AccountRepository
	txs      domain.TransactionRepository
}

// Footprints returns the assets and address count of each named ledger.
func (q *FootprintQueries) Footprints(ctx context.Context, ledgerNames []string) (map[string]ledger.Footprint, error) {
	if len(ledgerNames) == 0 {
		return map[string]ledger.Footprint{}, nil
	}
	out, err := q.accounts.LedgerFootprints(ctx, ledgerNames)
	if err != nil {
		return nil, fmt.Errorf("ledger footprints: %w", err)
	}
	return out, nil
}

// EventTypeUsage counts the transactions and holds of each event type name.
func (q *FootprintQueries) EventTypeUsage(ctx context.Context, ledgerName string) (map[string]int64, error) {
	usage, err := q.txs.EventTypeUsage(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("event type usage: %w", err)
	}
	return usage, nil
}

// AddressAssets lists the assets an address already has accounts in; none for an unknown address.
func (q *FootprintQueries) AddressAssets(ctx context.Context, ledgerName string, address ledger.Address) ([]string, error) {
	row, err := q.accounts.GetAddress(ctx, ledgerName, address)
	if errors.Is(err, ledger.ErrNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get address %q: %w", address, err)
	}
	out := make([]string, len(row.Accounts))
	for i := range row.Accounts {
		out[i] = row.Accounts[i].Asset
	}
	return out, nil
}
