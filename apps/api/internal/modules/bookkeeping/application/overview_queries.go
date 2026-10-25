package application

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

type overviewQueries struct {
	ledgers  LedgerDirectory
	accounts *accountQueries
	addrs    domain.AccountRepository
	writes   domain.WriteRepository
	txs      domain.TransactionRepository
	holds    domain.HoldRepository
	tb       domain.LedgerStore
	clock    ledger.Clock
}

// maxReservedLookup bounds the open holds read to sum reserved amounts.
const maxReservedLookup = LookupChunk

func (u *overviewQueries) Stats(ctx context.Context, ledgerName string) (Stats, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
		return Stats{}, err
	}
	now := u.clock.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var s Stats
	var err error
	if s.Accounts, s.AccountsToday, err = u.addrs.CountAddresses(ctx, ledgerName, midnight); err != nil {
		return Stats{}, fmt.Errorf("count addresses: %w", err)
	}
	if s.Assets, err = u.accounts.Assets(ctx, ledgerName); err != nil {
		return Stats{}, err
	}
	if s.Holds, err = u.holds.CountHolds(ctx, ledgerName, now); err != nil {
		return Stats{}, fmt.Errorf("count holds: %w", err)
	}
	open, err := u.holds.OpenHolds(ctx, ledgerName, now, maxReservedLookup)
	if err != nil {
		return Stats{}, fmt.Errorf("open holds: %w", err)
	}
	ids := make([]ledger.ID, len(open))
	assetByTransfer := make(map[ledger.ID]string, len(open))
	for i := range open {
		ids[i] = open[i].PendingTransferID
		assetByTransfer[open[i].PendingTransferID] = open[i].Source.Asset
	}
	s.Reserved = map[string]*big.Int{}
	if len(ids) > 0 {
		found, err := u.tb.LookupTransfers(ctx, ids)
		if err != nil {
			return Stats{}, fmt.Errorf("lookup transfers: %w", err)
		}
		for k := range found {
			t := &found[k]
			asset := assetByTransfer[t.ID]
			sum, ok := s.Reserved[asset]
			if !ok {
				sum = new(big.Int)
				s.Reserved[asset] = sum
			}
			sum.Add(sum, t.Amount.Big())
		}
	}
	if s.Declined, err = u.writes.DeclinedSince(ctx, ledgerName, now.Add(-24*time.Hour)); err != nil {
		return Stats{}, fmt.Errorf("declined writes: %w", err)
	}
	for _, d := range s.Declined {
		s.DeclinedTotal += d.Count
	}
	return s, nil
}

const searchLimit = 5

func (u *overviewQueries) Search(ctx context.Context, ledgerName, query string) (SearchResult, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
		return SearchResult{}, err
	}
	var out SearchResult
	var err error
	if out.Accounts, err = u.addrs.SearchAddresses(ctx, ledgerName, query, searchLimit); err != nil {
		return SearchResult{}, fmt.Errorf("search addresses: %w", err)
	}
	if out.Transactions, err = u.txs.SearchTransactions(ctx, ledgerName, query, searchLimit); err != nil {
		return SearchResult{}, fmt.Errorf("search transactions: %w", err)
	}
	if id, err := ledger.ParseID(query); err == nil {
		if h, err := u.holds.GetHold(ctx, ledgerName, id); err == nil {
			out.Holds = append(out.Holds, SearchHold{ID: h.ID, Status: domain.HoldStatusAt(h.Status, h.CreatedAt, h.TimeoutSeconds, u.clock.Now())})
		}
	}
	rows, err := u.writes.ListWrites(ctx, ledgerName, domain.WriteFilter{Search: query}, domain.WriteCursor{}, searchLimit)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search writes: %w", err)
	}
	out.Requests = rows
	return out, nil
}
