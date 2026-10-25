package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// LedgerCommands creates ledgers and edits their metadata.
type LedgerCommands interface {
	// Create validates the name and metadata and records the ledger. Metadata may be nil.
	Create(ctx context.Context, name string, metadata map[string]string) (LedgerView, error)
	ReplaceMetadata(ctx context.Context, name string, metadata map[string]string) (LedgerView, error)
}

// LedgerQueries reads ledgers.
type LedgerQueries interface {
	Get(ctx context.Context, name string) (LedgerView, error)
	// List returns a page of ledgers in name order and the cursor of the next page.
	List(ctx context.Context, cursor string, limit int) ([]LedgerView, string, error)
}

// SettingsCommands edits the account rules and event types of a ledger (ADR-0006, ADR-0007).
type SettingsCommands interface {
	// PutRule creates or replaces the rule for a pattern. It applies to accounts created afterwards.
	PutRule(ctx context.Context, ledgerName string, rule ledger.Rule) (ledger.Rule, error)
	// DeleteRule removes a stored rule; deleting a stored "*" or "world" restores the default.
	DeleteRule(ctx context.Context, ledgerName, pattern string) error
	// CreateEventType registers an event type. A zero code takes the next free one.
	CreateEventType(ctx context.Context, ledgerName, name string, code int) (ledger.EventType, error)
}

// SettingsQueries reads the account rules and event types of a ledger.
type SettingsQueries interface {
	// Rules returns the effective rules, most specific first, built-in defaults included.
	Rules(ctx context.Context, ledgerName string) ([]ledger.Rule, error)
	// MatchRule resolves the rule that would create the account for an address.
	MatchRule(ctx context.Context, ledgerName, address string) (RuleMatch, error)
	// EventTypes lists the registry, the built-in default first, with usage counts.
	EventTypes(ctx context.Context, ledgerName string) ([]EventTypeView, error)
}

// LedgerView is a ledger with the assets it uses and its address count.
type LedgerView struct {
	ledger.Ledger
	ledger.Footprint
}

// RuleMatch is the answer to "which rule wins for this address".
type RuleMatch struct {
	Address    ledger.Address
	Rule       ledger.Rule
	Candidates []ledger.RuleCandidate
	// Existing lists the assets the address already has accounts in; their flags are frozen.
	Existing []string
}

// EventTypeView is an event type with the transactions and holds that use it.
type EventTypeView struct {
	ledger.EventType
	Uses int64
}
