// Package application holds the use cases of the ledgers module: ledgers, account rules and
// event types (plan section 4.6). Commands and queries are separate types; usecases.go exports them
// as interfaces, which every transport depends on. ports.go declares what the module needs from
// the bookkeeping module, which bootstrap passes in: modules never import each other.
package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Authorizer is answered by the access module (ADR-0028): whether the caller holds a permission,
// on a ledger or in the organization of the call, and which ledgers it may see.
type Authorizer interface {
	Require(ctx context.Context, perm ledger.Permission, ledgerName string) error
	VisibleLedgers(ctx context.Context) (all bool, names []string, err error)
}

// Footprints is answered by bookkeeping: the assets and address count shown with each ledger, for
// a whole page of ledgers in one round trip. A ledger without accounts is absent from the map.
type Footprints interface {
	Footprints(ctx context.Context, ledgerNames []string) (map[string]ledger.Footprint, error)
}

// EventTypeUsage is answered by bookkeeping: the transactions and holds that use each event type
// name.
type EventTypeUsage interface {
	EventTypeUsage(ctx context.Context, ledgerName string) (map[string]int64, error)
}

// AddressAssets is answered by bookkeeping: the assets an address already has accounts in, for
// matchRule. An address without accounts has none.
type AddressAssets interface {
	AddressAssets(ctx context.Context, ledgerName string, address ledger.Address) ([]string, error)
}
