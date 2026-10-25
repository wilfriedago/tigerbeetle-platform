package app

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// Ledgers manages ledger namespaces (ADR-0004).
type Ledgers struct {
	store LedgerRepository
	clock Clock
}

// NewLedgers builds the use case.
func NewLedgers(store LedgerRepository, clock Clock) *Ledgers {
	return &Ledgers{store: store, clock: clock}
}

// Create validates the name and metadata and records the ledger. Metadata may be nil.
func (u *Ledgers) Create(ctx context.Context, name string, metadata map[string]string) (LedgerSummary, error) {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return LedgerSummary{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return LedgerSummary{}, err
	}
	created, err := u.store.CreateLedger(ctx, ledger.Ledger{
		Name:      name,
		Metadata:  metadata,
		CreatedAt: u.clock.Now().UTC(),
	})
	if err != nil {
		return LedgerSummary{}, fmt.Errorf("create ledger %q: %w", name, err)
	}
	return LedgerSummary{Ledger: created, Assets: []string{}}, nil
}

// Get returns a ledger by name.
func (u *Ledgers) Get(ctx context.Context, name string) (ledger.Ledger, error) {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return ledger.Ledger{}, err
	}
	l, err := u.store.GetLedger(ctx, name)
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("get ledger %q: %w", name, err)
	}
	return l, nil
}

// Summary returns a ledger with its assets and address count.
func (u *Ledgers) Summary(ctx context.Context, name string) (LedgerSummary, error) {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return LedgerSummary{}, err
	}
	s, err := u.store.LedgerSummary(ctx, name)
	if err != nil {
		return LedgerSummary{}, fmt.Errorf("get ledger %q: %w", name, err)
	}
	return s, nil
}

// List returns a page of ledgers in name order.
func (u *Ledgers) List(ctx context.Context, cursor string, limit int) ([]LedgerSummary, string, error) {
	after, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	limit = pageSize(limit)
	rows, err := u.store.ListLedgers(ctx, after, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list ledgers: %w", err)
	}
	page, next := trimPage(rows, limit, func(s LedgerSummary) string { return s.Name })
	return page, next, nil
}

// ReplaceMetadata replaces the metadata of a ledger.
func (u *Ledgers) ReplaceMetadata(ctx context.Context, name string, metadata map[string]string) (LedgerSummary, error) {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return LedgerSummary{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return LedgerSummary{}, err
	}
	if err := u.store.ReplaceLedgerMetadata(ctx, name, metadata); err != nil {
		return LedgerSummary{}, fmt.Errorf("replace metadata of ledger %q: %w", name, err)
	}
	return u.Summary(ctx, name)
}
