package application

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// Reconciler settles idempotency records left pending by a crash or a TigerBeetle timeout between
// "recorded in Postgres" and "accepted by TigerBeetle" (ADR-0007). It never mints new IDs: it
// looks the stored IDs up and either records what landed or resubmits the same chain.
type Reconciler struct {
	writes   domain.WriteRepository
	txs      *transactionCommands
	holds    *holdCommands
	clock    ledger.Clock
	grace    time.Duration
	interval time.Duration
	batch    int
	logger   *slog.Logger

	mu          sync.Mutex
	running     bool
	lastRunAt   time.Time
	lastSettled int
	stuck       map[string]StuckRecord
}

// newReconciler builds the reconciler. Run starts its loop.
func newReconciler(writes domain.WriteRepository, txs *transactionCommands, holds *holdCommands, clock ledger.Clock, grace, interval time.Duration, logger *slog.Logger) *Reconciler {
	if grace <= 0 {
		grace = DefaultGracePeriod
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Reconciler{writes: writes, txs: txs, holds: holds, clock: clock, grace: grace, interval: interval, batch: 100, logger: logger, stuck: map[string]StuckRecord{}}
}

// Run settles pending records every interval until ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		r.RunOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunOnce settles the pending records older than the grace period and returns how many it settled.
func (r *Reconciler) RunOnce(ctx context.Context) int {
	now := r.clock.Now()
	records, err := r.writes.PendingWrites(ctx, now.Add(-r.grace), r.batch)
	if err != nil {
		r.logger.WarnContext(ctx, "reconciler: list pending writes", slog.Any("error", err))
		return 0
	}
	settled := 0
	for i := range records {
		rec := &records[i]
		if ctx.Err() != nil {
			break
		}
		// The record's tenant, so every read and write of the settlement stays in its organization.
		ok, err := r.settle(ledger.WithTenant(ctx, ledger.Tenant{ID: rec.Organization}), *rec)
		id := rec.Organization.String() + "/" + rec.Ledger + "/" + rec.Key
		r.mu.Lock()
		if ok {
			settled++
			delete(r.stuck, id)
		} else {
			note := "a chain cannot be partially present; check the transfer IDs"
			if err != nil {
				note = "TigerBeetle or Postgres did not answer; retrying"
			}
			r.stuck[id] = StuckRecord{Ledger: rec.Ledger, Key: rec.Key, Operation: rec.Operation, CreatedAt: rec.CreatedAt, Attempts: rec.Attempts, Note: note}
		}
		r.mu.Unlock()
		if err != nil {
			r.logger.WarnContext(ctx, "reconciler: settle write", slog.String("ledger", rec.Ledger), slog.String("idempotency_key", rec.Key), slog.Any("error", err))
		} else if ok {
			r.logger.InfoContext(ctx, "reconciler: settled write", slog.String("ledger", rec.Ledger), slog.String("idempotency_key", rec.Key), slog.String("operation", string(rec.Operation)))
		}
	}
	r.mu.Lock()
	r.lastRunAt = r.clock.Now()
	r.lastSettled = settled
	r.mu.Unlock()
	return settled
}

func (r *Reconciler) settle(ctx context.Context, rec domain.WriteRecord) (bool, error) {
	switch rec.Operation {
	case domain.OperationTransaction, domain.OperationRevert:
		return r.txs.settlePending(ctx, rec)
	default:
		return r.holds.settlePending(ctx, rec)
	}
}

// Status reports the last run and the records the reconciler is waiting on.
func (r *Reconciler) Status(ctx context.Context) (ReconcilerStatus, error) {
	pending, stuck, err := r.writes.CountPending(ctx, r.clock.Now().Add(-r.grace))
	if err != nil {
		return ReconcilerStatus{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s := ReconcilerStatus{
		Running:     r.running,
		Grace:       r.grace,
		Interval:    r.interval,
		LastRunAt:   r.lastRunAt,
		LastSettled: r.lastSettled,
		Pending:     pending,
		Stuck:       stuck,
	}
	for _, rec := range r.stuck {
		s.Records = append(s.Records, rec)
	}
	return s, nil
}
