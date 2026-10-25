package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

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

func holdRow(c holdColumns) (domain.HoldRow, error) {
	md, err := platformpg.DecodeMetadata(c.Metadata)
	if err != nil {
		return domain.HoldRow{}, err
	}
	return domain.HoldRow{
		ID:                   platformpg.IDOf(c.ID),
		Ledger:               c.Ledger,
		PendingTransferID:    platformpg.IDOf(c.PendingTransferID),
		Source:               accountRow(c.Account),
		Destination:          accountRow(c.Account_2),
		Reference:            platformpg.StringOf(c.Reference),
		EventType:            c.EventType,
		Code:                 uint16(c.Code),
		Metadata:             md,
		TimeoutSeconds:       uint32(c.TimeoutSeconds),
		IdempotencyKey:       c.IdempotencyKey,
		RequestID:            platformpg.StringOf(c.RequestID),
		CreatedAt:            platformpg.TimeOf(c.CreatedAt),
		TBTimestamp:          uint64(c.TbTimestamp),
		Status:               domain.HoldStatus(c.Status),
		ResolutionTransferID: platformpg.OptionalID(c.ResolutionTransferID),
		ResolvedAt:           platformpg.OptionalTime(c.ResolvedAt),
	}, nil
}

// GetHold returns a hold with its accounts.
func (s *Store) GetHold(ctx context.Context, ledgerName string, id ledger.ID) (domain.HoldRow, error) {
	r, err := s.queries.GetHold(ctx, gen.GetHoldParams{Ledger: ledgerName, ID: platformpg.UUIDOf(id)})
	if err != nil {
		return domain.HoldRow{}, fmt.Errorf("postgres: get hold: %w", mapError(err))
	}
	return holdRow(holdColumns(r))
}

// ListHolds returns holds newest first.
func (s *Store) ListHolds(ctx context.Context, ledgerName string, filter domain.HoldFilter, before ledger.ID, limit int, now time.Time) ([]domain.HoldRow, error) {
	rows, err := s.queries.ListHolds(ctx, gen.ListHoldsParams{
		Ledger:   ledgerName,
		BeforeID: platformpg.NullableUUID(before),
		Status:   platformpg.OptionalString(string(filter.Status)),
		Now:      platformpg.Timestamptz(now),
		Prefix:   platformpg.OptionalString(string(filter.Address)),
		PageSize: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list holds: %w", mapError(err))
	}
	out := make([]domain.HoldRow, 0, len(rows))
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
func (s *Store) CountHolds(ctx context.Context, ledgerName string, now time.Time) (domain.HoldCounts, error) {
	r, err := s.queries.CountHolds(ctx, gen.CountHoldsParams{Ledger: ledgerName, Now: platformpg.Timestamptz(now)})
	if err != nil {
		return domain.HoldCounts{}, fmt.Errorf("postgres: count holds: %w", mapError(err))
	}
	return domain.HoldCounts{Open: r.Open, Captured: r.Captured, Voided: r.Voided, Expired: r.Expired}, nil
}

// OpenHolds returns open, unexpired holds.
func (s *Store) OpenHolds(ctx context.Context, ledgerName string, now time.Time, limit int) ([]domain.HoldRow, error) {
	rows, err := s.queries.OpenHolds(ctx, gen.OpenHoldsParams{Ledger: ledgerName, Now: platformpg.Timestamptz(now), MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("postgres: open holds: %w", mapError(err))
	}
	out := make([]domain.HoldRow, 0, len(rows))
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
	if err := s.queries.MarkHoldExpired(ctx, gen.MarkHoldExpiredParams{Ledger: ledgerName, ID: platformpg.UUIDOf(id)}); err != nil {
		return fmt.Errorf("postgres: mark hold expired: %w", mapError(err))
	}
	return nil
}

// CountOpenHoldsFrom counts the open holds whose source is the address.
func (s *Store) CountOpenHoldsFrom(ctx context.Context, ledgerName string, address ledger.Address, now time.Time) (int64, error) {
	n, err := s.queries.CountOpenHoldsFrom(ctx, gen.CountOpenHoldsFromParams{Ledger: ledgerName, Address: string(address), Now: platformpg.Timestamptz(now)})
	if err != nil {
		return 0, fmt.Errorf("postgres: count open holds: %w", mapError(err))
	}
	return n, nil
}
