package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/domain"
)

type ledgerQueries struct {
	ledgers    domain.LedgerRepository
	footprints Footprints
}

func (u *ledgerQueries) Get(ctx context.Context, name string) (LedgerView, error) {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return LedgerView{}, err
	}
	l, err := u.ledgers.GetLedger(ctx, name)
	if err != nil {
		return LedgerView{}, fmt.Errorf("get ledger %q: %w", name, err)
	}
	views, err := u.views(ctx, []ledger.Ledger{l})
	if err != nil {
		return LedgerView{}, err
	}
	return views[0], nil
}

func (u *ledgerQueries) List(ctx context.Context, cursor string, limit int) ([]LedgerView, string, error) {
	after, err := ledger.DecodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	limit = ledger.PageSize(limit)
	rows, err := u.ledgers.ListLedgers(ctx, after, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list ledgers: %w", err)
	}
	page, next := ledger.TrimPage(rows, limit, func(l ledger.Ledger) string { return l.Name })
	views, err := u.views(ctx, page)
	if err != nil {
		return nil, "", err
	}
	return views, next, nil
}

// views adds the footprint of every ledger, read from bookkeeping in one call for the page.
func (u *ledgerQueries) views(ctx context.Context, ledgers []ledger.Ledger) ([]LedgerView, error) {
	names := make([]string, len(ledgers))
	for i := range ledgers {
		names[i] = ledgers[i].Name
	}
	footprints, err := u.footprints.Footprints(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("ledger footprints: %w", err)
	}
	out := make([]LedgerView, len(ledgers))
	for i := range ledgers {
		f := footprints[ledgers[i].Name]
		if f.Assets == nil {
			f.Assets = []string{}
		}
		out[i] = LedgerView{Ledger: ledgers[i], Footprint: f}
	}
	return out, nil
}
