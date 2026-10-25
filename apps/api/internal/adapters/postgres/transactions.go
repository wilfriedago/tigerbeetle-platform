package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
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

func transactionRow(c transactionColumns) (app.TransactionRow, error) {
	md, err := decodeMetadata(c.Metadata)
	if err != nil {
		return app.TransactionRow{}, err
	}
	return app.TransactionRow{
		ID:             idOf(c.ID),
		Ledger:         c.Ledger,
		Reference:      stringOf(c.Reference),
		EventType:      c.EventType,
		Code:           uint16(c.Code),
		BusinessTime:   optionalTime(c.BusinessTime),
		Metadata:       md,
		IdempotencyKey: c.IdempotencyKey,
		RequestID:      stringOf(c.RequestID),
		Reverts:        optionalID(c.Reverts),
		RevertedBy:     optionalID(c.RevertedBy),
		CommittedAt:    timeOf(c.CommittedAt),
		TBTimestamp:    uint64(c.TbTimestamp),
	}, nil
}

// withLegs loads the postings of the given transactions and attaches them in chain order.
func (s *Store) withLegs(ctx context.Context, rows []app.TransactionRow) ([]app.TransactionRow, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	ids := make([]pgtype.UUID, len(rows))
	index := make(map[ledger.ID]int, len(rows))
	for i := range rows {
		r := &rows[i]
		ids[i] = uuidOf(r.ID)
		index[r.ID] = i
	}
	legs, err := s.queries.TransactionPostings(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("postgres: transaction postings: %w", mapError(err))
	}
	for k := range legs {
		l := &legs[k]
		i := index[idOf(l.TransactionID)]
		rows[i].Legs = append(rows[i].Legs, app.LegRow{TransferID: idOf(l.TransferID), Source: accountRow(l.Account), Destination: accountRow(l.Account_2)})
	}
	return rows, nil
}

// GetTransaction returns a transaction with its legs.
func (s *Store) GetTransaction(ctx context.Context, ledgerName string, id ledger.ID) (app.TransactionRow, error) {
	r, err := s.queries.GetTransaction(ctx, gen.GetTransactionParams{Ledger: ledgerName, ID: uuidOf(id)})
	if err != nil {
		return app.TransactionRow{}, fmt.Errorf("postgres: get transaction: %w", mapError(err))
	}
	row, err := transactionRow(transactionColumns(r))
	if err != nil {
		return app.TransactionRow{}, err
	}
	rows, err := s.withLegs(ctx, []app.TransactionRow{row})
	if err != nil {
		return app.TransactionRow{}, err
	}
	return rows[0], nil
}

// ListTransactions returns transactions newest first, before the given ID.
func (s *Store) ListTransactions(ctx context.Context, ledgerName string, filter app.TransactionFilter, before ledger.ID, limit int) ([]app.TransactionRow, error) {
	rows, err := s.queries.ListTransactions(ctx, gen.ListTransactionsParams{
		Ledger:          ledgerName,
		BeforeID:        nullableUUID(before),
		Reference:       optionalString(filter.Reference),
		EventType:       optionalString(filter.EventType),
		CommittedAfter:  optionalTimestamptz(filter.After),
		CommittedBefore: optionalTimestamptz(filter.Before),
		Prefix:          optionalString(string(filter.Address)),
		Asset:           optionalString(filter.Asset),
		PageSize:        int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list transactions: %w", mapError(err))
	}
	out := make([]app.TransactionRow, 0, len(rows))
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
	raw, err := encodeMetadata(metadata)
	if err != nil {
		return err
	}
	n, err := s.queries.ReplaceTransactionMetadata(ctx, gen.ReplaceTransactionMetadataParams{Ledger: ledgerName, ID: uuidOf(id), Metadata: raw})
	if err != nil {
		return fmt.Errorf("postgres: replace transaction metadata: %w", mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: replace transaction metadata: %w", ledger.ErrNotFound)
	}
	return nil
}

// SearchTransactions finds transactions by ID or reference prefix.
func (s *Store) SearchTransactions(ctx context.Context, ledgerName, text string, limit int) ([]app.TransactionRow, error) {
	params := gen.SearchTransactionsParams{Ledger: ledgerName, Text: text, MaxRows: int32(limit)}
	if id, err := ledger.ParseID(text); err == nil {
		params.ID = uuidOf(id)
	}
	rows, err := s.queries.SearchTransactions(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("postgres: search transactions: %w", mapError(err))
	}
	out := make([]app.TransactionRow, 0, len(rows))
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

// holdColumns is the shape every hold query returns.
type holdColumns struct {
	ID                   pgtype.UUID
	Ledger               string
	PendingTransferID    pgtype.UUID
	Reference            *string
	EventType            string
	Code                 int32
	Metadata             []byte
	TimeoutSeconds       int64
	IdempotencyKey       string
	RequestID            *string
	CreatedAt            pgtype.Timestamptz
	TbTimestamp          int64
	Status               string
	ResolutionTransferID pgtype.UUID
	ResolvedAt           pgtype.Timestamptz
	Account              gen.Account
	Account_2            gen.Account
}

func holdRow(c holdColumns) (app.HoldRow, error) {
	md, err := decodeMetadata(c.Metadata)
	if err != nil {
		return app.HoldRow{}, err
	}
	return app.HoldRow{
		ID:                   idOf(c.ID),
		Ledger:               c.Ledger,
		PendingTransferID:    idOf(c.PendingTransferID),
		Source:               accountRow(c.Account),
		Destination:          accountRow(c.Account_2),
		Reference:            stringOf(c.Reference),
		EventType:            c.EventType,
		Code:                 uint16(c.Code),
		Metadata:             md,
		TimeoutSeconds:       uint32(c.TimeoutSeconds),
		IdempotencyKey:       c.IdempotencyKey,
		RequestID:            stringOf(c.RequestID),
		CreatedAt:            timeOf(c.CreatedAt),
		TBTimestamp:          uint64(c.TbTimestamp),
		Status:               ledger.HoldStatus(c.Status),
		ResolutionTransferID: optionalID(c.ResolutionTransferID),
		ResolvedAt:           optionalTime(c.ResolvedAt),
	}, nil
}

// GetHold returns a hold with its accounts.
func (s *Store) GetHold(ctx context.Context, ledgerName string, id ledger.ID) (app.HoldRow, error) {
	r, err := s.queries.GetHold(ctx, gen.GetHoldParams{Ledger: ledgerName, ID: uuidOf(id)})
	if err != nil {
		return app.HoldRow{}, fmt.Errorf("postgres: get hold: %w", mapError(err))
	}
	return holdRow(holdColumns(r))
}

// ListHolds returns holds newest first.
func (s *Store) ListHolds(ctx context.Context, ledgerName string, filter app.HoldFilter, before ledger.ID, limit int, now time.Time) ([]app.HoldRow, error) {
	rows, err := s.queries.ListHolds(ctx, gen.ListHoldsParams{
		Ledger:   ledgerName,
		BeforeID: nullableUUID(before),
		Status:   optionalString(string(filter.Status)),
		Now:      timestamptz(now),
		Prefix:   optionalString(string(filter.Address)),
		PageSize: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list holds: %w", mapError(err))
	}
	out := make([]app.HoldRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		row, err := holdRow(holdColumns(*r))
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// CountHolds counts holds by visible status.
func (s *Store) CountHolds(ctx context.Context, ledgerName string, now time.Time) (app.HoldCounts, error) {
	r, err := s.queries.CountHolds(ctx, gen.CountHoldsParams{Ledger: ledgerName, Now: timestamptz(now)})
	if err != nil {
		return app.HoldCounts{}, fmt.Errorf("postgres: count holds: %w", mapError(err))
	}
	return app.HoldCounts{Open: r.Open, Captured: r.Captured, Voided: r.Voided, Expired: r.Expired}, nil
}

// OpenHolds returns open, unexpired holds.
func (s *Store) OpenHolds(ctx context.Context, ledgerName string, now time.Time, limit int) ([]app.HoldRow, error) {
	rows, err := s.queries.OpenHolds(ctx, gen.OpenHoldsParams{Ledger: ledgerName, Now: timestamptz(now), MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("postgres: open holds: %w", mapError(err))
	}
	out := make([]app.HoldRow, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		row, err := holdRow(holdColumns(*r))
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// MarkHoldExpired records that TigerBeetle released the hold.
func (s *Store) MarkHoldExpired(ctx context.Context, ledgerName string, id ledger.ID) error {
	if err := s.queries.MarkHoldExpired(ctx, gen.MarkHoldExpiredParams{Ledger: ledgerName, ID: uuidOf(id)}); err != nil {
		return fmt.Errorf("postgres: mark hold expired: %w", mapError(err))
	}
	return nil
}

// CountOpenHoldsFrom counts the open holds whose source is the address.
func (s *Store) CountOpenHoldsFrom(ctx context.Context, ledgerName string, address ledger.Address, now time.Time) (int64, error) {
	n, err := s.queries.CountOpenHoldsFrom(ctx, gen.CountOpenHoldsFromParams{Ledger: ledgerName, Address: string(address), Now: timestamptz(now)})
	if err != nil {
		return 0, fmt.Errorf("postgres: count open holds: %w", mapError(err))
	}
	return n, nil
}
