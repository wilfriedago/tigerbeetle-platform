package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jackc/pgx/v5"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

func writeRecord(w gen.WriteRequest) domain.WriteRecord {
	rec := domain.WriteRecord{
		Organization: platformpg.IDOf(w.OrganizationID),
		Ledger:       w.Ledger,
		Key:          w.IdempotencyKey,
		Operation:    domain.Operation(w.Operation),
		Status:       domain.RequestStatus(w.Status),
		BodyHash:     w.BodyHash,
		Request:      w.Request,
		TargetID:     platformpg.IDOf(w.TargetID),
		TransferIDs:  platformpg.IDsOf(w.TransferIds),
		Response:     w.Response,
		ProblemType:  platformpg.StringOf(w.ProblemType),
		RequestID:    platformpg.StringOf(w.RequestID),
		Attempts:     int(w.Attempts),
		CreatedAt:    platformpg.TimeOf(w.CreatedAt),
		ResolvedAt:   platformpg.OptionalTime(w.ResolvedAt),
		ActorIssuer:  platformpg.StringOf(w.ActorIssuer),
		ActorSubject: platformpg.StringOf(w.ActorSubject),
	}
	if w.ResponseStatus != nil {
		rec.ResponseStatus = int(*w.ResponseStatus)
	}
	return rec
}

func writeRecords(ws []gen.WriteRequest) []domain.WriteRecord {
	out := make([]domain.WriteRecord, len(ws))
	for i := range ws {
		w := &ws[i]
		out[i] = writeRecord(*w)
	}
	return out
}

// BeginWrite inserts a pending record, or returns the existing record for the key.
func (s *Store) BeginWrite(ctx context.Context, rec domain.WriteRecord) (domain.WriteRecord, bool, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return domain.WriteRecord{}, false, err
	}
	row, err := s.queries.InsertWrite(ctx, gen.InsertWriteParams{
		OrganizationID: org,
		Ledger:         rec.Ledger,
		IdempotencyKey: rec.Key,
		Operation:      string(rec.Operation),
		BodyHash:       rec.BodyHash,
		Request:        rec.Request,
		TargetID:       platformpg.UUIDOf(rec.TargetID),
		TransferIds:    platformpg.UUIDsOf(rec.TransferIDs),
		RequestID:      platformpg.OptionalString(rec.RequestID),
		CreatedAt:      platformpg.Timestamptz(rec.CreatedAt),
		ActorIssuer:    platformpg.OptionalString(rec.ActorIssuer),
		ActorSubject:   platformpg.OptionalString(rec.ActorSubject),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING returned nothing: the key exists.
		existing, err := s.GetWrite(ctx, rec.Ledger, rec.Key)
		if err != nil {
			return domain.WriteRecord{}, false, err
		}
		return existing, false, nil
	}
	if err != nil {
		return domain.WriteRecord{}, false, fmt.Errorf("postgres: insert write: %w", mapError(err))
	}
	return writeRecord(row), true, nil
}

// GetWrite returns a record by key.
func (s *Store) GetWrite(ctx context.Context, ledgerName, key string) (domain.WriteRecord, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return domain.WriteRecord{}, err
	}
	row, err := s.queries.GetWrite(ctx, gen.GetWriteParams{OrganizationID: org, Ledger: ledgerName, IdempotencyKey: key})
	if err != nil {
		return domain.WriteRecord{}, fmt.Errorf("postgres: get write: %w", mapError(err))
	}
	return writeRecord(row), nil
}

// ListWrites returns records newest first.
func (s *Store) ListWrites(ctx context.Context, ledgerName string, filter domain.WriteFilter, cursor domain.WriteCursor, limit int) ([]domain.WriteRecord, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	params := gen.ListWritesParams{
		OrganizationID: org,
		Ledger:         ledgerName,
		Status:         platformpg.OptionalString(string(filter.Status)),
		Operation:      platformpg.OptionalString(string(filter.Operation)),
		Search:         platformpg.OptionalString(filter.Search),
		Actor:          platformpg.OptionalString(filter.Actor),
		PageSize:       int32(limit),
	}
	if !cursor.CreatedAt.IsZero() {
		params.CursorAt = platformpg.Timestamptz(cursor.CreatedAt)
		params.CursorKey = &cursor.Key
	}
	rows, err := s.queries.ListWrites(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("postgres: list writes: %w", mapError(err))
	}
	return writeRecords(rows), nil
}

// NoteAttempt counts a TigerBeetle submission.
func (s *Store) NoteAttempt(ctx context.Context, ledgerName, key string) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	if err := s.queries.NoteAttempt(ctx, gen.NoteAttemptParams{OrganizationID: org, Ledger: ledgerName, IdempotencyKey: key}); err != nil {
		return fmt.Errorf("postgres: note attempt: %w", mapError(err))
	}
	return nil
}

func resolveParams(org pgtype.UUID, o domain.WriteOutcome) gen.ResolveWriteParams {
	status := int32(o.ResponseStatus)
	return gen.ResolveWriteParams{
		OrganizationID: org,
		Ledger:         o.Ledger,
		IdempotencyKey: o.Key,
		Status:         string(o.Status),
		ResponseStatus: &status,
		Response:       o.Response,
		ProblemType:    platformpg.OptionalString(o.ProblemType),
		ResolvedAt:     platformpg.Timestamptz(o.ResolvedAt),
	}
}

// ResolveWrite records a declined or failed outcome. A record that is no longer pending was
// resolved by a concurrent request or the reconciler; that is not an error.
func (s *Store) ResolveWrite(ctx context.Context, ledgerName, key string, outcome domain.WriteOutcome) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	outcome.Ledger, outcome.Key = ledgerName, key
	if _, err := s.queries.ResolveWrite(ctx, resolveParams(org, outcome)); err != nil {
		return fmt.Errorf("postgres: resolve write: %w", mapError(err))
	}
	return nil
}

// commit resolves the record and runs insert in the same transaction. When the record was already
// committed, by the reconciler for example, nothing is inserted twice.
func (s *Store) commit(ctx context.Context, outcome domain.WriteOutcome, insert func(q *gen.Queries) error) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *gen.Queries) error {
		n, err := q.ResolveWrite(ctx, resolveParams(org, outcome))
		if err != nil {
			return err
		}
		if n == 0 {
			current, err := q.GetWrite(ctx, gen.GetWriteParams{OrganizationID: org, Ledger: outcome.Ledger, IdempotencyKey: outcome.Key})
			if err != nil {
				return err
			}
			if current.Status == string(domain.RequestCommitted) {
				return nil
			}
			return fmt.Errorf("write %q is %s, not pending", outcome.Key, current.Status)
		}
		return insert(q)
	})
}

// CommitTransaction resolves the record and inserts the transaction with its postings.
func (s *Store) CommitTransaction(ctx context.Context, outcome domain.WriteOutcome, tx domain.TransactionRow) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	metadata, err := platformpg.EncodeMetadata(tx.Metadata)
	if err != nil {
		return err
	}
	var reverts ledger.ID
	if tx.Reverts != nil {
		reverts = *tx.Reverts
	}
	postings := gen.InsertPostingsParams{TransactionID: platformpg.UUIDOf(tx.ID)}
	for i := range tx.Legs {
		leg := &tx.Legs[i]
		postings.Positions = append(postings.Positions, int16(i))
		postings.TransferIds = append(postings.TransferIds, platformpg.UUIDOf(leg.TransferID))
		postings.Sources = append(postings.Sources, platformpg.UUIDOf(leg.Source.ID))
		postings.Destinations = append(postings.Destinations, platformpg.UUIDOf(leg.Destination.ID))
	}
	err = s.commit(ctx, outcome, func(q *gen.Queries) error {
		if err := q.InsertTransaction(ctx, gen.InsertTransactionParams{
			OrganizationID: org,
			ID:             platformpg.UUIDOf(tx.ID),
			Ledger:         tx.Ledger,
			Reference:      platformpg.OptionalString(tx.Reference),
			EventType:      tx.EventType,
			Code:           int32(tx.Code),
			BusinessTime:   platformpg.OptionalTimestamptz(tx.BusinessTime),
			Metadata:       metadata,
			IdempotencyKey: tx.IdempotencyKey,
			RequestID:      platformpg.OptionalString(tx.RequestID),
			Reverts:        platformpg.NullableUUID(reverts),
			CommittedAt:    platformpg.Timestamptz(tx.CommittedAt),
			TbTimestamp:    int64(tx.TBTimestamp),
		}); err != nil {
			return err
		}
		return q.InsertPostings(ctx, postings)
	})
	if err != nil {
		return fmt.Errorf("postgres: commit transaction: %w", mapError(err))
	}
	return nil
}

// CommitHold resolves the record and inserts the hold.
func (s *Store) CommitHold(ctx context.Context, outcome domain.WriteOutcome, hold domain.HoldRow) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	metadata, err := platformpg.EncodeMetadata(hold.Metadata)
	if err != nil {
		return err
	}
	err = s.commit(ctx, outcome, func(q *gen.Queries) error {
		return q.InsertHold(ctx, gen.InsertHoldParams{
			OrganizationID:     org,
			ID:                 platformpg.UUIDOf(hold.ID),
			Ledger:             hold.Ledger,
			PendingTransferID:  platformpg.UUIDOf(hold.PendingTransferID),
			SourceAccount:      platformpg.UUIDOf(hold.Source.ID),
			DestinationAccount: platformpg.UUIDOf(hold.Destination.ID),
			Reference:          platformpg.OptionalString(hold.Reference),
			EventType:          hold.EventType,
			Code:               int32(hold.Code),
			Metadata:           metadata,
			TimeoutSeconds:     int64(hold.TimeoutSeconds),
			IdempotencyKey:     hold.IdempotencyKey,
			RequestID:          platformpg.OptionalString(hold.RequestID),
			CreatedAt:          platformpg.Timestamptz(hold.CreatedAt),
			TbTimestamp:        int64(hold.TBTimestamp),
		})
	})
	if err != nil {
		return fmt.Errorf("postgres: commit hold: %w", mapError(err))
	}
	return nil
}

// CommitResolution resolves the record and the hold.
func (s *Store) CommitResolution(ctx context.Context, outcome domain.WriteOutcome, r domain.HoldResolution) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	err = s.commit(ctx, outcome, func(q *gen.Queries) error {
		_, err := q.ResolveHold(ctx, gen.ResolveHoldParams{
			OrganizationID:       org,
			Ledger:               r.Ledger,
			ID:                   platformpg.UUIDOf(r.HoldID),
			Status:               string(r.Status),
			ResolutionTransferID: platformpg.UUIDOf(r.ResolutionTransferID),
			ResolvedAt:           platformpg.Timestamptz(r.ResolvedAt),
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("postgres: commit hold resolution: %w", mapError(err))
	}
	return nil
}

// PendingWrites returns pending records created before a time, oldest first.
func (s *Store) PendingWrites(ctx context.Context, before time.Time, limit int) ([]domain.WriteRecord, error) {
	rows, err := s.queries.PendingWrites(ctx, gen.PendingWritesParams{Before: platformpg.Timestamptz(before), MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("postgres: pending writes: %w", mapError(err))
	}
	return writeRecords(rows), nil
}

// CountPending counts pending records and those older than a time.
func (s *Store) CountPending(ctx context.Context, before time.Time) (pending, stuck int64, err error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return 0, 0, err
	}
	r, err := s.queries.CountPending(ctx, gen.CountPendingParams{OrganizationID: org, Before: platformpg.Timestamptz(before)})
	if err != nil {
		return 0, 0, fmt.Errorf("postgres: count pending: %w", mapError(err))
	}
	return r.Pending, r.Stuck, nil
}

// DeclinedSince groups declined and failed writes by problem type.
func (s *Store) DeclinedSince(ctx context.Context, ledgerName string, since time.Time) ([]domain.ReasonCount, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.DeclinedSince(ctx, gen.DeclinedSinceParams{OrganizationID: org, Ledger: ledgerName, Since: platformpg.Timestamptz(since)})
	if err != nil {
		return nil, fmt.Errorf("postgres: declined writes: %w", mapError(err))
	}
	out := make([]domain.ReasonCount, len(rows))
	for i, r := range rows {
		out[i] = domain.ReasonCount{Reason: r.Reason, Count: r.Occurrences}
	}
	return out, nil
}
