package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

type transactionQueries struct {
	ledgers LedgerDirectory
	txs     domain.TransactionRepository
	tb      domain.LedgerStore
}

func transactionView(row domain.TransactionRow, postings []domain.Posting) TransactionView {
	v := TransactionView{
		ID:           row.ID,
		Ledger:       row.Ledger,
		Reference:    row.Reference,
		EventType:    ledger.EventType{Name: row.EventType, Code: row.Code, BuiltIn: row.EventType == ledger.DefaultEventType.Name},
		BusinessTime: row.BusinessTime,
		Metadata:     row.Metadata,
		// The TigerBeetle timestamp has nanoseconds; Postgres keeps microseconds.
		CommittedAt:    domain.TimeFromTB(row.TBTimestamp),
		Timestamp:      row.TBTimestamp,
		Reverts:        row.Reverts,
		RevertedBy:     row.RevertedBy,
		IdempotencyKey: row.IdempotencyKey,
		RequestID:      row.RequestID,
	}
	for i := range row.Legs {
		leg := &row.Legs[i]
		v.Postings = append(v.Postings, PostingView{
			Source:      leg.Source.Address,
			Destination: leg.Destination.Address,
			Asset:       leg.Source.Asset,
			Amount:      postings[i].Amount,
			TransferID:  leg.TransferID,
			TBLedger:    leg.Source.TBLedger,
		})
	}
	return v
}

// views reads the amounts of the transactions from TigerBeetle in one batch.
func (u *transactionQueries) views(ctx context.Context, rows []domain.TransactionRow) ([]TransactionView, error) {
	var ids []ledger.ID
	for i := range rows {
		legs := rows[i].Legs
		for k := range legs {
			ids = append(ids, legs[k].TransferID)
		}
	}
	amounts := make(map[ledger.ID]ledger.Amount, len(ids))
	for start := 0; start < len(ids); start += LookupChunk {
		found, err := u.tb.LookupTransfers(ctx, ids[start:min(start+LookupChunk, len(ids))])
		if err != nil {
			return nil, fmt.Errorf("lookup transfers: %w", err)
		}
		for i := range found {
			amounts[found[i].ID] = found[i].Amount
		}
	}
	out := make([]TransactionView, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		postings := make([]domain.Posting, len(r.Legs))
		for k := range r.Legs {
			l := &r.Legs[k]
			amount, ok := amounts[l.TransferID]
			if !ok {
				return nil, fmt.Errorf("%w: transfer %s of transaction %s is missing in TigerBeetle", ledger.ErrUnavailable, l.TransferID, r.ID)
			}
			postings[k] = domain.Posting{Amount: amount}
		}
		out = append(out, transactionView(*r, postings))
	}
	return out, nil
}

func (u *transactionQueries) Get(ctx context.Context, ledgerName, id string) (TransactionView, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
		return TransactionView{}, err
	}
	txID, err := ledger.ParseID(id)
	if err != nil {
		return TransactionView{}, err
	}
	row, err := u.txs.GetTransaction(ctx, ledgerName, txID)
	if err != nil {
		return TransactionView{}, fmt.Errorf("get transaction %s: %w", txID, err)
	}
	views, err := u.views(ctx, []domain.TransactionRow{row})
	if err != nil {
		return TransactionView{}, err
	}
	return views[0], nil
}

func (u *transactionQueries) List(ctx context.Context, in ListTransactionsInput) ([]TransactionView, string, error) {
	if err := u.ledgers.Require(ctx, in.Ledger); err != nil {
		return nil, "", err
	}
	filter := domain.TransactionFilter{Reference: in.Reference, EventType: in.EventType, After: in.After, Before: in.Before}
	p, err := parsePrefix(in.Address)
	if err != nil {
		return nil, "", err
	}
	filter.Address = p
	if in.Asset != "" {
		a, err := ledger.ParseAsset(in.Asset)
		if err != nil {
			return nil, "", err
		}
		filter.Asset = a.String()
	}
	before, err := decodeIDCursor(in.Cursor)
	if err != nil {
		return nil, "", err
	}
	limit := ledger.PageSize(in.Limit)
	rows, err := u.txs.ListTransactions(ctx, in.Ledger, filter, before, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list transactions: %w", err)
	}
	page, next := ledger.TrimPage(rows, limit, func(r domain.TransactionRow) string { return r.ID.String() })
	views, err := u.views(ctx, page)
	if err != nil {
		return nil, "", err
	}
	return views, next, nil
}
