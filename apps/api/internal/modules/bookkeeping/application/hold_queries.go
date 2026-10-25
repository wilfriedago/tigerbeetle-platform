package application

import (
	"context"
	"fmt"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

type holdQueries struct {
	ledgers LedgerDirectory
	holds   domain.HoldRepository
	tb      domain.LedgerStore
	clock   ledger.Clock
}

func (u *holdQueries) view(row domain.HoldRow, amount ledger.Amount, captured *ledger.Amount) HoldView {
	v := HoldView{
		ID:                   row.ID,
		Ledger:               row.Ledger,
		Source:               row.Source.Address,
		Destination:          row.Destination.Address,
		Asset:                row.Source.Asset,
		Amount:               amount,
		CapturedAmount:       captured,
		Status:               domain.HoldStatusAt(row.Status, row.CreatedAt, row.TimeoutSeconds, u.clock.Now()),
		TimeoutSeconds:       row.TimeoutSeconds,
		CreatedAt:            domain.TimeFromTB(row.TBTimestamp),
		Timestamp:            row.TBTimestamp,
		ResolvedAt:           row.ResolvedAt,
		Reference:            row.Reference,
		EventType:            ledger.EventType{Name: row.EventType, Code: row.Code, BuiltIn: row.EventType == ledger.DefaultEventType.Name},
		Metadata:             row.Metadata,
		IdempotencyKey:       row.IdempotencyKey,
		PendingTransferID:    row.PendingTransferID,
		ResolutionTransferID: row.ResolutionTransferID,
		TBLedger:             row.Source.TBLedger,
	}
	if row.TimeoutSeconds > 0 {
		expires := v.CreatedAt.Add(time.Duration(row.TimeoutSeconds) * time.Second)
		v.ExpiresAt = &expires
	}
	return v
}

// views reads hold and capture amounts from TigerBeetle in one batch.
func (u *holdQueries) views(ctx context.Context, rows []domain.HoldRow) ([]HoldView, error) {
	ids := make([]ledger.ID, 0, 2*len(rows))
	for i := range rows {
		ids = append(ids, rows[i].PendingTransferID)
		if rows[i].ResolutionTransferID != nil {
			ids = append(ids, *rows[i].ResolutionTransferID)
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
	out := make([]HoldView, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		amount, ok := amounts[r.PendingTransferID]
		if !ok {
			return nil, fmt.Errorf("%w: pending transfer of hold %s is missing in TigerBeetle", ledger.ErrUnavailable, r.ID)
		}
		var captured *ledger.Amount
		if r.Status == domain.HoldCaptured && r.ResolutionTransferID != nil {
			if c, ok := amounts[*r.ResolutionTransferID]; ok {
				captured = &c
			}
		}
		out = append(out, u.view(*r, amount, captured))
	}
	return out, nil
}

func (u *holdQueries) Get(ctx context.Context, ledgerName, id string) (HoldView, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
		return HoldView{}, err
	}
	holdID, err := ledger.ParseID(id)
	if err != nil {
		return HoldView{}, err
	}
	row, err := u.holds.GetHold(ctx, ledgerName, holdID)
	if err != nil {
		return HoldView{}, fmt.Errorf("get hold %s: %w", holdID, err)
	}
	views, err := u.views(ctx, []domain.HoldRow{row})
	if err != nil {
		return HoldView{}, err
	}
	return views[0], nil
}

func (u *holdQueries) List(ctx context.Context, in ListHoldsInput) ([]HoldView, string, domain.HoldCounts, error) {
	if err := u.ledgers.Require(ctx, in.Ledger); err != nil {
		return nil, "", domain.HoldCounts{}, err
	}
	filter := domain.HoldFilter{Status: domain.HoldStatus(in.Status)}
	switch filter.Status {
	case "", domain.HoldOpen, domain.HoldCaptured, domain.HoldVoided, domain.HoldExpired:
	default:
		return nil, "", domain.HoldCounts{}, fmt.Errorf("%w: status must be open, captured, voided or expired", ledger.ErrInvalid)
	}
	p, err := parsePrefix(in.Address)
	if err != nil {
		return nil, "", domain.HoldCounts{}, err
	}
	filter.Address = p
	before, err := decodeIDCursor(in.Cursor)
	if err != nil {
		return nil, "", domain.HoldCounts{}, err
	}
	limit := ledger.PageSize(in.Limit)
	now := u.clock.Now()
	rows, err := u.holds.ListHolds(ctx, in.Ledger, filter, before, limit+1, now)
	if err != nil {
		return nil, "", domain.HoldCounts{}, fmt.Errorf("list holds: %w", err)
	}
	page, next := ledger.TrimPage(rows, limit, func(r domain.HoldRow) string { return r.ID.String() })
	views, err := u.views(ctx, page)
	if err != nil {
		return nil, "", domain.HoldCounts{}, err
	}
	counts, err := u.holds.CountHolds(ctx, in.Ledger, now)
	if err != nil {
		return nil, "", domain.HoldCounts{}, fmt.Errorf("count holds: %w", err)
	}
	return views, next, counts, nil
}

func (u *holdQueries) Counts(ctx context.Context, ledgerName string) (domain.HoldCounts, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
		return domain.HoldCounts{}, err
	}
	counts, err := u.holds.CountHolds(ctx, ledgerName, u.clock.Now())
	if err != nil {
		return domain.HoldCounts{}, fmt.Errorf("count holds: %w", err)
	}
	return counts, nil
}
