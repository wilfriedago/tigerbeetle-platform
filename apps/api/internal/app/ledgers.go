package app

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// Ledgers manages ledger namespaces. It exists to prove the Postgres path end to end; the
// HTTP endpoints for it arrive with the ledger API.
type Ledgers struct {
	store MetadataStore
	clock Clock
}

// NewLedgers builds the use case.
func NewLedgers(store MetadataStore, clock Clock) *Ledgers {
	return &Ledgers{store: store, clock: clock}
}

// Create validates the name and records the ledger. Metadata may be nil.
func (u *Ledgers) Create(ctx context.Context, name string, metadata map[string]string) (ledger.Ledger, error) {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return ledger.Ledger{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	created, err := u.store.CreateLedger(ctx, ledger.Ledger{
		Name:      name,
		Metadata:  metadata,
		CreatedAt: u.clock.Now().UTC(),
	})
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("create ledger %q: %w", name, err)
	}
	return created, nil
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
