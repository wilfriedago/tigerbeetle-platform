package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func writeRecord(w gen.WriteRequest) app.WriteRecord {
	rec := app.WriteRecord{
		Ledger:      w.Ledger,
		Key:         w.IdempotencyKey,
		Operation:   ledger.Operation(w.Operation),
		Status:      ledger.RequestStatus(w.Status),
		BodyHash:    w.BodyHash,
		Request:     w.Request,
		TargetID:    idOf(w.TargetID),
		TransferIDs: idsOf(w.TransferIds),
		Response:    w.Response,
		ProblemType: stringOf(w.ProblemType),
		RequestID:   stringOf(w.RequestID),
		Attempts:    int(w.Attempts),
		CreatedAt:   timeOf(w.CreatedAt),
		ResolvedAt:  optionalTime(w.ResolvedAt),
	}
	if w.ResponseStatus != nil {
		rec.ResponseStatus = int(*w.ResponseStatus)
	}
	return rec
}

func writeRecords(ws []gen.WriteRequest) []app.WriteRecord {
	out := make([]app.WriteRecord, len(ws))
	for i := range ws {
		w := &ws[i]
		out[i] = writeRecord(*w)
	}
	return out
}

// BeginWrite inserts a pending record, or returns the existing record for the key.
func (s *Store) BeginWrite(ctx context.Context, rec app.WriteRecord) (app.WriteRecord, bool, error) {
	row, err := s.queries.InsertWrite(ctx, gen.InsertWriteParams{
		Ledger:         rec.Ledger,
		IdempotencyKey: rec.Key,
		Operation:      string(rec.Operation),
		BodyHash:       rec.BodyHash,
		Request:        rec.Request,
		TargetID:       uuidOf(rec.TargetID),
		TransferIds:    uuidsOf(rec.TransferIDs),
		RequestID:      optionalString(rec.RequestID),
		CreatedAt:      timestamptz(rec.CreatedAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING returned nothing: the key exists.
		existing, err := s.GetWrite(ctx, rec.Ledger, rec.Key)
		if err != nil {
			return app.WriteRecord{}, false, err
		}
		return existing, false, nil
	}
	if err != nil {
		return app.WriteRecord{}, false, fmt.Errorf("postgres: insert write: %w", mapError(err))
	}
	return writeRecord(row), true, nil
}

// GetWrite returns a record by key.
func (s *Store) GetWrite(ctx context.Context, ledgerName, key string) (app.WriteRecord, error) {
	row, err := s.queries.GetWrite(ctx, gen.GetWriteParams{Ledger: ledgerName, IdempotencyKey: key})
	if err != nil {
		return app.WriteRecord{}, fmt.Errorf("postgres: get write: %w", mapError(err))
	}
	return writeRecord(row), nil
}

// ListWrites returns records newest first.
func (s *Store) ListWrites(ctx context.Context, ledgerName string, filter app.WriteFilter, cursor app.WriteCursor, limit int) ([]app.WriteRecord, error) {
	params := gen.ListWritesParams{
		Ledger:    ledgerName,
		Status:    optionalString(string(filter.Status)),
		Operation: optionalString(string(filter.Operation)),
		Search:    optionalString(filter.Search),
		PageSize:  int32(limit),
	}
	if !cursor.CreatedAt.IsZero() {
		params.CursorAt = timestamptz(cursor.CreatedAt)
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
	if err := s.queries.NoteAttempt(ctx, gen.NoteAttemptParams{Ledger: ledgerName, IdempotencyKey: key}); err != nil {
		return fmt.Errorf("postgres: note attempt: %w", mapError(err))
	}
	return nil
}

func resolveParams(o app.WriteOutcome) gen.ResolveWriteParams {
	status := int32(o.ResponseStatus)
	return gen.ResolveWriteParams{
		Ledger:         o.Ledger,
		IdempotencyKey: o.Key,
		Status:         string(o.Status),
		ResponseStatus: &status,
		Response:       o.Response,
		ProblemType:    optionalString(o.ProblemType),
		ResolvedAt:     timestamptz(o.ResolvedAt),
	}
}

// ResolveWrite records a declined or failed outcome. A record that is no longer pending was
// resolved by a concurrent request or the reconciler; that is not an error.
func (s *Store) ResolveWrite(ctx context.Context, ledgerName, key string, outcome app.WriteOutcome) error {
	outcome.Ledger, outcome.Key = ledgerName, key
	if _, err := s.queries.ResolveWrite(ctx, resolveParams(outcome)); err != nil {
		return fmt.Errorf("postgres: resolve write: %w", mapError(err))
	}
	return nil
}

// commit resolves the record and runs insert in the same transaction. When the record was already
// committed, by the reconciler for example, nothing is inserted twice.
func (s *Store) commit(ctx context.Context, outcome app.WriteOutcome, insert func(q *gen.Queries) error) error {
	return s.inTx(ctx, func(q *gen.Queries) error {
		n, err := q.ResolveWrite(ctx, resolveParams(outcome))
		if err != nil {
			return err
		}
		if n == 0 {
			current, err := q.GetWrite(ctx, gen.GetWriteParams{Ledger: outcome.Ledger, IdempotencyKey: outcome.Key})
			if err != nil {
				return err
			}
			if current.Status == string(ledger.RequestCommitted) {
				return nil
			}
			return fmt.Errorf("write %q is %s, not pending", outcome.Key, current.Status)
		}
		return insert(q)
	})
}

// CommitTransaction resolves the record and inserts the transaction with its postings.
func (s *Store) CommitTransaction(ctx context.Context, outcome app.WriteOutcome, tx app.TransactionRow) error {
	metadata, err := encodeMetadata(tx.Metadata)
	if err != nil {
		return err
	}
	var reverts ledger.ID
	if tx.Reverts != nil {
		reverts = *tx.Reverts
	}
	postings := gen.InsertPostingsParams{TransactionID: uuidOf(tx.ID)}
	for i := range tx.Legs {
		leg := &tx.Legs[i]
		postings.Positions = append(postings.Positions, int16(i))
		postings.TransferIds = append(postings.TransferIds, uuidOf(leg.TransferID))
		postings.Sources = append(postings.Sources, uuidOf(leg.Source.ID))
		postings.Destinations = append(postings.Destinations, uuidOf(leg.Destination.ID))
	}
	err = s.commit(ctx, outcome, func(q *gen.Queries) error {
		if err := q.InsertTransaction(ctx, gen.InsertTransactionParams{
			ID:             uuidOf(tx.ID),
			Ledger:         tx.Ledger,
			Reference:      optionalString(tx.Reference),
			EventType:      tx.EventType,
			Code:           int32(tx.Code),
			BusinessTime:   optionalTimestamptz(tx.BusinessTime),
			Metadata:       metadata,
			IdempotencyKey: tx.IdempotencyKey,
			RequestID:      optionalString(tx.RequestID),
			Reverts:        nullableUUID(reverts),
			CommittedAt:    timestamptz(tx.CommittedAt),
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
func (s *Store) CommitHold(ctx context.Context, outcome app.WriteOutcome, hold app.HoldRow) error {
	metadata, err := encodeMetadata(hold.Metadata)
	if err != nil {
		return err
	}
	err = s.commit(ctx, outcome, func(q *gen.Queries) error {
		return q.InsertHold(ctx, gen.InsertHoldParams{
			ID:                 uuidOf(hold.ID),
			Ledger:             hold.Ledger,
			PendingTransferID:  uuidOf(hold.PendingTransferID),
			SourceAccount:      uuidOf(hold.Source.ID),
			DestinationAccount: uuidOf(hold.Destination.ID),
			Reference:          optionalString(hold.Reference),
			EventType:          hold.EventType,
			Code:               int32(hold.Code),
			Metadata:           metadata,
			TimeoutSeconds:     int64(hold.TimeoutSeconds),
			IdempotencyKey:     hold.IdempotencyKey,
			RequestID:          optionalString(hold.RequestID),
			CreatedAt:          timestamptz(hold.CreatedAt),
			TbTimestamp:        int64(hold.TBTimestamp),
		})
	})
	if err != nil {
		return fmt.Errorf("postgres: commit hold: %w", mapError(err))
	}
	return nil
}

// CommitResolution resolves the record and the hold.
func (s *Store) CommitResolution(ctx context.Context, outcome app.WriteOutcome, r app.HoldResolution) error {
	err := s.commit(ctx, outcome, func(q *gen.Queries) error {
		_, err := q.ResolveHold(ctx, gen.ResolveHoldParams{
			Ledger:               r.Ledger,
			ID:                   uuidOf(r.HoldID),
			Status:               string(r.Status),
			ResolutionTransferID: uuidOf(r.ResolutionTransferID),
			ResolvedAt:           timestamptz(r.ResolvedAt),
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("postgres: commit hold resolution: %w", mapError(err))
	}
	return nil
}

// PendingWrites returns pending records created before a time, oldest first.
func (s *Store) PendingWrites(ctx context.Context, before time.Time, limit int) ([]app.WriteRecord, error) {
	rows, err := s.queries.PendingWrites(ctx, gen.PendingWritesParams{Before: timestamptz(before), MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("postgres: pending writes: %w", mapError(err))
	}
	return writeRecords(rows), nil
}

// CountPending counts pending records and those older than a time.
func (s *Store) CountPending(ctx context.Context, before time.Time) (pending, stuck int64, err error) {
	r, err := s.queries.CountPending(ctx, timestamptz(before))
	if err != nil {
		return 0, 0, fmt.Errorf("postgres: count pending: %w", mapError(err))
	}
	return r.Pending, r.Stuck, nil
}

// DeclinedSince groups declined and failed writes by problem type.
func (s *Store) DeclinedSince(ctx context.Context, ledgerName string, since time.Time) ([]app.ReasonCount, error) {
	rows, err := s.queries.DeclinedSince(ctx, gen.DeclinedSinceParams{Ledger: ledgerName, Since: timestamptz(since)})
	if err != nil {
		return nil, fmt.Errorf("postgres: declined writes: %w", mapError(err))
	}
	out := make([]app.ReasonCount, len(rows))
	for i, r := range rows {
		out[i] = app.ReasonCount{Reason: r.Reason, Count: r.Occurrences}
	}
	return out, nil
}
