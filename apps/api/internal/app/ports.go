package app

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// LedgerStore is the TigerBeetle port: the only store of balances and transfers (ADR-0002).
// Account and transfer operations will be added here as the ledger use cases arrive.
type LedgerStore interface {
	// Ready performs a round trip to the cluster and fails when it does not answer before the
	// context deadline.
	Ready(ctx context.Context) error
}

// MetadataStore is the Postgres port: names, metadata, idempotency records and the outbox
// (ADR-0002). It never stores a balance.
type MetadataStore interface {
	Ready(ctx context.Context) error
	// CreateLedger inserts a ledger and returns ledger.ErrAlreadyExists on a name collision.
	CreateLedger(ctx context.Context, l ledger.Ledger) (ledger.Ledger, error)
	// GetLedger returns ledger.ErrNotFound when no ledger has that name.
	GetLedger(ctx context.Context, name string) (ledger.Ledger, error)
}

// Clock abstracts time so use cases are deterministic in tests.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function such as time.Now to the Clock port.
type ClockFunc func() time.Time

// Now calls the wrapped function.
func (f ClockFunc) Now() time.Time { return f() }

// IDGenerator mints time-ordered 128-bit identifiers. The TigerBeetle adapter implements it with
// the client's ID() helper (ADR-0006).
type IDGenerator interface {
	NewID() ledger.ID
}
