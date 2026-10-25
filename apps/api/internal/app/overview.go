package app

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// Overview answers the ledger overview, the write request log and search.
type Overview struct {
	ledgers  LedgerRepository
	accounts *Accounts
	addrs    AccountRepository
	writes   WriteRepository
	txs      TransactionRepository
	holds    HoldRepository
	tb       LedgerStore
	clock    Clock
}

// NewOverview builds the use case.
func NewOverview(store MetadataStore, accounts *Accounts, tb LedgerStore, clock Clock) *Overview {
	return &Overview{ledgers: store, accounts: accounts, addrs: store, writes: store, txs: store, holds: store, tb: tb, clock: clock}
}

// Stats are the overview figures of a ledger.
type Stats struct {
	Accounts      int64
	AccountsToday int64
	Assets        []AssetView
	Holds         HoldCounts
	// Reserved is the amount held by open holds, per asset, read from TigerBeetle.
	Reserved      map[string]*big.Int
	DeclinedTotal int64
	Declined      []ReasonCount
}

// maxReservedLookup bounds the open holds read to sum reserved amounts.
const maxReservedLookup = LookupChunk

// Stats computes the overview figures.
func (u *Overview) Stats(ctx context.Context, ledgerName string) (Stats, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
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
		h := &open[i]
		ids[i] = h.PendingTransferID
		assetByTransfer[h.PendingTransferID] = h.Source.Asset
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

// Requests lists the idempotency records of a ledger, newest first.
func (u *Overview) Requests(ctx context.Context, ledgerName, status, operation, search, cursor string, limit int) ([]WriteRecord, string, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return nil, "", err
	}
	filter := WriteFilter{Status: ledger.RequestStatus(status), Operation: ledger.Operation(operation), Search: search}
	after, err := decodeWriteCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	limit = pageSize(limit)
	rows, err := u.writes.ListWrites(ctx, ledgerName, filter, after, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list writes: %w", err)
	}
	if len(rows) <= limit {
		return rows, "", nil
	}
	page := rows[:limit]
	last := page[len(page)-1]
	return page, encodeWriteCursor(WriteCursor{CreatedAt: last.CreatedAt, Key: last.Key}), nil
}

// Request returns one idempotency record.
func (u *Overview) Request(ctx context.Context, ledgerName, key string) (WriteRecord, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return WriteRecord{}, err
	}
	rec, err := u.writes.GetWrite(ctx, ledgerName, key)
	if err != nil {
		return WriteRecord{}, fmt.Errorf("get write %q: %w", key, err)
	}
	return rec, nil
}

// SearchResult groups what a query names.
type SearchResult struct {
	Accounts     []AddressRow
	Transactions []TransactionRow
	Holds        []HoldRow
	Requests     []WriteRecord
}

const searchLimit = 5

// Search finds accounts by address prefix, transactions by ID or reference, holds by ID, and write
// requests by Idempotency-Key.
func (u *Overview) Search(ctx context.Context, ledgerName, query string) (SearchResult, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
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
			out.Holds = append(out.Holds, h)
		}
	}
	if rows, err := u.writes.ListWrites(ctx, ledgerName, WriteFilter{Search: query}, WriteCursor{}, searchLimit); err == nil {
		out.Requests = rows
	} else {
		return SearchResult{}, fmt.Errorf("search writes: %w", err)
	}
	return out, nil
}

// HoldStatusNow derives the visible status of a stored hold.
func (u *Overview) HoldStatusNow(h HoldRow) ledger.HoldStatus {
	return ledger.HoldStatusAt(h.Status, h.CreatedAt, h.TimeoutSeconds, u.clock.Now())
}
