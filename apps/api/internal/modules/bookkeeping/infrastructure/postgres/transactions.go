package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// transactionColumns is the shape every transaction query returns.
type transactionColumns struct {
	ID             pgtype.UUID
	Ledger         string
	Reference      *string
	EventType      string
	Code           int32
	BusinessTime   pgtype.Timestamptz
	Metadata       []byte
	IdempotencyKey string
	RequestID      *string
	Reverts        pgtype.UUID
	CommittedAt    pgtype.Timestamptz
	TbTimestamp    int64
	RevertedBy     pgtype.UUID
}

func transactionRow(c transactionColumns) (domain.TransactionRow, error) {
	md, err := platformpg.DecodeMetadata(c.Metadata)
	if err != nil {
		return domain.TransactionRow{}, err
	}
	return domain.TransactionRow{
		ID:             platformpg.IDOf(c.ID),
		Ledger:         c.Ledger,
		Reference:      platformpg.StringOf(c.Reference),
		EventType:      c.EventType,
		Code:           uint16(c.Code),
		BusinessTime:   platformpg.OptionalTime(c.BusinessTime),
		Metadata:       md,
		IdempotencyKey: c.IdempotencyKey,
		RequestID:      platformpg.StringOf(c.RequestID),
		Reverts:        platformpg.OptionalID(c.Reverts),
		RevertedBy:     platformpg.OptionalID(c.RevertedBy),
		CommittedAt:    platformpg.TimeOf(c.CommittedAt),
		TBTimestamp:    uint64(c.TbTimestamp),
	}, nil
}

// withLegs loads the postings of the given transactions and attaches them in chain order.
func (s *Store) withLegs(ctx context.Context, rows []domain.TransactionRow) ([]domain.TransactionRow, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	ids := make([]pgtype.UUID, len(rows))
	index := make(map[ledger.ID]int, len(rows))
	for i := range rows {
		r := &rows[i]
		ids[i] = platformpg.UUIDOf(r.ID)
		index[r.ID] = i
	}
	legs, err := s.queries.TransactionPostings(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("postgres: transaction postings: %w", mapError(err))
	}
	for k := range legs {
		l := &legs[k]
		i := index[platformpg.IDOf(l.TransactionID)]
		rows[i].Legs = append(rows[i].Legs, domain.LegRow{TransferID: platformpg.IDOf(l.TransferID), Source: accountRow(l.Account), Destination: accountRow(l.Account_2)})
	}
	return rows, nil
}

// GetTransaction returns a transaction with its legs.
func (s *Store) GetTransaction(ctx context.Context, ledgerName string, id ledger.ID) (domain.TransactionRow, error) {
	r, err := s.queries.GetTransaction(ctx, gen.GetTransactionParams{Ledger: ledgerName, ID: platformpg.UUIDOf(id)})
	if err != nil {
		return domain.TransactionRow{}, fmt.Errorf("postgres: get transaction: %w", mapError(err))
	}
	row, err := transactionRow(transactionColumns(r))
	if err != nil {
		return domain.TransactionRow{}, err
	}
	rows, err := s.withLegs(ctx, []domain.TransactionRow{row})
	if err != nil {
		return domain.TransactionRow{}, err
	}
	return rows[0], nil
}

// ListTransactions returns transactions newest first, before the given ID.
func (s *Store) ListTransactions(ctx context.Context, ledgerName string, filter domain.TransactionFilter, before ledger.ID, limit int) ([]domain.TransactionRow, error) {
	rows, err := s.queries.ListTransactions(ctx, gen.ListTransactionsParams{
		Ledger:          ledgerName,
		BeforeID:        platformpg.NullableUUID(before),
		Reference:       platformpg.OptionalString(filter.Reference),
		EventType:       platformpg.OptionalString(filter.EventType),
		CommittedAfter:  platformpg.OptionalTimestamptz(filter.After),
		CommittedBefore: platformpg.OptionalTimestamptz(filter.Before),
		Prefix:          platformpg.OptionalString(string(filter.Address)),
		Asset:           platformpg.OptionalString(filter.Asset),
		PageSize:        int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list transactions: %w", mapError(err))
	}
	out := make([]domain.TransactionRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		row, err := transactionRow(transactionColumns(*r))
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return s.withLegs(ctx, out)
}

// ReplaceTransactionMetadata replaces the metadata of a transaction.
func (s *Store) ReplaceTransactionMetadata(ctx context.Context, ledgerName string, id ledger.ID, metadata map[string]string) error {
	raw, err := platformpg.EncodeMetadata(metadata)
	if err != nil {
		return err
	}
	n, err := s.queries.ReplaceTransactionMetadata(ctx, gen.ReplaceTransactionMetadataParams{Ledger: ledgerName, ID: platformpg.UUIDOf(id), Metadata: raw})
	if err != nil {
		return fmt.Errorf("postgres: replace transaction metadata: %w", mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: replace transaction metadata: %w", ledger.ErrNotFound)
	}
	return nil
}

// SearchTransactions finds transactions by ID or reference prefix.
func (s *Store) SearchTransactions(ctx context.Context, ledgerName, text string, limit int) ([]domain.TransactionRow, error) {
	params := gen.SearchTransactionsParams{Ledger: ledgerName, Text: text, MaxRows: int32(limit)}
	if id, err := ledger.ParseID(text); err == nil {
		params.ID = platformpg.UUIDOf(id)
	}
	rows, err := s.queries.SearchTransactions(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("postgres: search transactions: %w", mapError(err))
	}
	out := make([]domain.TransactionRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		row, err := transactionRow(transactionColumns(*r))
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}
