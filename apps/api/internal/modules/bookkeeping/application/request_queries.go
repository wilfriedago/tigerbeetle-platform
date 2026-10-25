package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

type requestQueries struct {
	ledgers LedgerDirectory
	writes  domain.WriteRepository
}

func (u *requestQueries) List(ctx context.Context, in ListRequestsInput) ([]domain.WriteRecord, string, error) {
	if err := u.ledgers.Require(ctx, in.Ledger); err != nil {
		return nil, "", err
	}
	filter := domain.WriteFilter{Status: domain.RequestStatus(in.Status), Operation: domain.Operation(in.Operation), Search: in.Search}
	after, err := decodeWriteCursor(in.Cursor)
	if err != nil {
		return nil, "", err
	}
	limit := ledger.PageSize(in.Limit)
	rows, err := u.writes.ListWrites(ctx, in.Ledger, filter, after, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list writes: %w", err)
	}
	if len(rows) <= limit {
		return rows, "", nil
	}
	page := rows[:limit]
	last := page[len(page)-1]
	return page, encodeWriteCursor(domain.WriteCursor{CreatedAt: last.CreatedAt, Key: last.Key}), nil
}

func (u *requestQueries) Get(ctx context.Context, ledgerName, key string) (domain.WriteRecord, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
		return domain.WriteRecord{}, err
	}
	rec, err := u.writes.GetWrite(ctx, ledgerName, key)
	if err != nil {
		return domain.WriteRecord{}, fmt.Errorf("get write %q: %w", key, err)
	}
	return rec, nil
}
