package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// DefaultGracePeriod is how long a pending record is treated as a write in progress before the
// reconciler settles it (ADR-0007). It is well above the request timeout.
const DefaultGracePeriod = 30 * time.Second

// canonicalRequest marshals the normalized request and hashes it. encoding/json writes struct
// fields in declaration order and map keys sorted, so equal requests hash equally, whatever
// transport they came through (plan section 10).
func canonicalRequest(v any) (body, hash []byte, err error) {
	body, err = json.Marshal(v)
	if err != nil {
		return nil, nil, fmt.Errorf("encode request: %w", err)
	}
	sum := sha256.Sum256(body)
	return body, sum[:], nil
}

// writeFlow is the protocol shared by every write that moves money (ADR-0007): record first with
// minted IDs, then submit to TigerBeetle, then resolve the record.
type writeFlow struct {
	writes domain.WriteRepository
	tb     domain.LedgerStore
	clock  ledger.Clock
	grace  time.Duration
	logger *slog.Logger
}

// begin inserts the pending record. For an existing key it either replays (committed, returning
// the record and fresh false with a nil error), or answers with the problem the key maps to.
func (f *writeFlow) begin(ctx context.Context, rec domain.WriteRecord) (stored domain.WriteRecord, fresh bool, err error) {
	if p, ok := ledger.PrincipalFrom(ctx); ok {
		rec.ActorIssuer, rec.ActorSubject = p.Issuer, p.Subject
	}
	got, created, err := f.writes.BeginWrite(ctx, rec)
	if err != nil {
		return domain.WriteRecord{}, false, fmt.Errorf("begin write: %w", err)
	}
	if created {
		return got, true, nil
	}
	// A key belongs to the principal that used it first (ADR-0030). Another principal learns
	// only that the key is taken, never the stored answer.
	if got.ActorIssuer != rec.ActorIssuer || got.ActorSubject != rec.ActorSubject {
		return domain.WriteRecord{}, false, ledger.NewProblem(ledger.ErrRejected, "idempotency-key-reused",
			fmt.Sprintf("Idempotency-Key %q was already used by another caller; choose a new key", rec.Key))
	}
	if got.Operation != rec.Operation || !bytes.Equal(got.BodyHash, rec.BodyHash) {
		return got, false, ledger.NewProblem(ledger.ErrRejected, "idempotency-key-reused",
			fmt.Sprintf("Idempotency-Key %q was already used with a different request (%s); a key maps to exactly one write", rec.Key, got.Operation))
	}
	switch got.Status {
	case domain.RequestPending:
		retry := time.Second
		if age := f.clock.Now().Sub(got.CreatedAt); age > f.grace {
			retry = 5 * time.Second
		}
		return got, false, &ledger.Problem{
			Kind:       ledger.ErrConflict,
			Reason:     "request-in-flight",
			Detail:     "a write with this Idempotency-Key is still in progress; retry after the delay to get its outcome",
			Posting:    -1,
			RetryAfter: retry,
		}
	case domain.RequestCommitted:
		return got, false, nil
	default:
		return got, false, domain.StoredProblem(got)
	}
}

// submit records the attempt and sends the transfers. A TigerBeetle failure leaves the record
// pending: the request may still land, and the reconciler settles it either way.
func (f *writeFlow) submit(ctx context.Context, rec domain.WriteRecord, transfers []domain.TransferSpec) ([]domain.TransferOutcome, error) {
	if err := f.writes.NoteAttempt(ctx, rec.Ledger, rec.Key); err != nil {
		return nil, fmt.Errorf("note attempt: %w", err)
	}
	outcomes, err := f.tb.CreateTransfers(ctx, transfers)
	if err != nil {
		return nil, f.pending(ctx, rec, err)
	}
	if len(outcomes) != len(transfers) {
		return nil, fmt.Errorf("create_transfers returned %d results for %d transfers", len(outcomes), len(transfers))
	}
	return outcomes, nil
}

// pending answers a write whose record stays pending because a store did not answer. The request
// may still land; the reconciler settles the record either way, and a retry with the same key
// returns its outcome. Errors that are not outages pass through unchanged.
func (f *writeFlow) pending(ctx context.Context, rec domain.WriteRecord, err error) error {
	var p *ledger.Problem
	if !errors.Is(err, ledger.ErrUnavailable) || errors.As(err, &p) {
		return err
	}
	f.logger.WarnContext(ctx, "write interrupted; record left pending for the reconciler",
		slog.String("ledger", rec.Ledger), slog.String("idempotency_key", rec.Key), slog.Any("error", err))
	return &ledger.Problem{
		Kind:       ledger.ErrUnavailable,
		Reason:     "write-pending",
		Detail:     "TigerBeetle or Postgres did not answer in time. The write is recorded and will be completed or settled by the reconciler; retry with the same Idempotency-Key to get its outcome",
		Posting:    -1,
		RetryAfter: 5 * time.Second,
	}
}

// decline resolves the record as declined (a valid request TigerBeetle refused) or failed.
func (f *writeFlow) decline(ctx context.Context, rec domain.WriteRecord, status domain.RequestStatus, p *ledger.Problem) error {
	code, body := domain.ProblemOutcome(p)
	err := f.writes.ResolveWrite(ctx, rec.Ledger, rec.Key, domain.WriteOutcome{
		Ledger:         rec.Ledger,
		Key:            rec.Key,
		Status:         status,
		ResponseStatus: code,
		Response:       body,
		ProblemType:    p.Reason,
		ResolvedAt:     f.clock.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("resolve write: %w", err)
	}
	return p
}

// committedOutcome is the record update for a successful write.
func (f *writeFlow) committedOutcome(rec domain.WriteRecord, status int, id ledger.ID) domain.WriteOutcome {
	return domain.WriteOutcome{
		Ledger:         rec.Ledger,
		Key:            rec.Key,
		Status:         domain.RequestCommitted,
		ResponseStatus: status,
		Response:       domain.CommittedBody(id),
		ResolvedAt:     f.clock.Now().UTC(),
	}
}

// latestTimestamp reads the committed time of transfers that already existed.
func latestTimestamp(ctx context.Context, tb domain.LedgerStore, ids []ledger.ID) (uint64, error) {
	found, err := tb.LookupTransfers(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("lookup transfers: %w", err)
	}
	var ts uint64
	for i := range found {
		ts = max(ts, found[i].Timestamp)
	}
	if ts == 0 {
		return 0, fmt.Errorf("%w: committed transfers not found in TigerBeetle", ledger.ErrUnavailable)
	}
	return ts, nil
}
