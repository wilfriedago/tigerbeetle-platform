// Package application holds the use cases of the bookkeeping module: the write path of ADR-0007
// for transactions, reverts and holds, the reads of accounts, transactions, holds and write
// requests, the overview, and the reconciler (plan section 4.6). Commands and queries are separate
// types; usecases.go exports them as interfaces, which every transport depends on. ports.go
// declares what the module needs from the ledgers module, which bootstrap passes in: modules never
// import each other.
package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// LedgerDirectory is answered by the ledgers module.
type LedgerDirectory interface {
	// Require returns an error matching ledger.ErrInvalid for a malformed name and
	// ledger.ErrNotFound when the ledger does not exist.
	Require(ctx context.Context, name string) error
}

// AccountPolicies resolves the rule that creates the account of each address (ADR-0006).
// Bookkeeping freezes the result on the account row; the ledgers module owns the rules.
type AccountPolicies interface {
	PoliciesFor(ctx context.Context, ledgerName string, addresses []ledger.Address) (map[ledger.Address]ledger.Rule, error)
}

// EventTypes resolves an event type name to its code. An empty name is the built-in default; an
// unknown name is an invalid request.
type EventTypes interface {
	Resolve(ctx context.Context, ledgerName, name string) (ledger.EventType, error)
}
