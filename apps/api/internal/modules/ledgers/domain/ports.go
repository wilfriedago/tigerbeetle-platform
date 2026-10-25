// Package domain is the domain layer of the ledgers module: the repository ports its
// infrastructure implements (plan section 4.6). Ledgers, account rules and event types are shared
// kernel types, because the bookkeeping module reads them too; this package declares how the
// module stores them. It imports only the kernel.
package domain

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// LedgerRepository stores ledgers.
type LedgerRepository interface {
	// CreateLedger inserts a ledger and returns ledger.ErrAlreadyExists on a name collision.
	CreateLedger(ctx context.Context, l ledger.Ledger) (ledger.Ledger, error)
	// GetLedger returns ledger.ErrNotFound when no ledger has that name.
	GetLedger(ctx context.Context, name string) (ledger.Ledger, error)
	// ListLedgers returns ledgers in name order after the given name.
	ListLedgers(ctx context.Context, after string, limit int) ([]ledger.Ledger, error)
	// ReplaceLedgerMetadata returns ledger.ErrNotFound when no ledger has that name.
	ReplaceLedgerMetadata(ctx context.Context, name string, metadata map[string]string) error
}

// RuleRepository stores account rules. Built-in rules are not stored.
type RuleRepository interface {
	ListRules(ctx context.Context, ledgerName string) ([]ledger.Rule, error)
	PutRule(ctx context.Context, ledgerName string, rule ledger.Rule) (ledger.Rule, error)
	// DeleteRule returns ledger.ErrNotFound when no rule is stored for the pattern.
	DeleteRule(ctx context.Context, ledgerName string, pattern ledger.Pattern) error
}

// EventTypeRepository stores the append-only event type registry. It does not know the built-in
// default event type.
type EventTypeRepository interface {
	// ListEventTypes returns the stored event types in code order.
	ListEventTypes(ctx context.Context, ledgerName string) ([]ledger.EventType, error)
	// GetEventType returns ledger.ErrNotFound for unknown names.
	GetEventType(ctx context.Context, ledgerName, name string) (ledger.EventType, error)
	// CreateEventType inserts a type. A zero code means "the next free code". A taken name or code
	// returns ledger.ErrAlreadyExists.
	CreateEventType(ctx context.Context, ledgerName, name string, code uint16) (ledger.EventType, error)
}
