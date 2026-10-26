package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// The use cases behind their permission checks (ADR-0028). Each type wraps one use-case interface
// and checks the permission before it calls the real use case; New returns only these, so every
// transport gets the same answer. TestEveryUseCaseIsChecked fails when a method has no check.
//
// Go note: a type with the same methods as an interface, holding another value of that
// interface, is the decorator pattern. Go has no annotations, so this replaces @PreAuthorize.

type authorizedLedgerCommands struct {
	auth Authorizer
	next LedgerCommands
}

func (a authorizedLedgerCommands) Create(ctx context.Context, name string, metadata map[string]string) (LedgerView, error) {
	if err := a.auth.Require(ctx, ledger.PermOrgLedgers, ""); err != nil {
		return LedgerView{}, err
	}
	return a.next.Create(ctx, name, metadata)
}

func (a authorizedLedgerCommands) ReplaceMetadata(ctx context.Context, name string, metadata map[string]string) (LedgerView, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerConfigure, name); err != nil {
		return LedgerView{}, err
	}
	return a.next.ReplaceMetadata(ctx, name, metadata)
}

type authorizedLedgerQueries struct {
	auth Authorizer
	next LedgerQueries
}

func (a authorizedLedgerQueries) Get(ctx context.Context, name string) (LedgerView, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, name); err != nil {
		return LedgerView{}, err
	}
	return a.next.Get(ctx, name)
}

// List needs no permission of its own: it lists only the ledgers the caller may read, which the
// query asks the authorizer for (ledgerQueries.visible).
func (a authorizedLedgerQueries) List(ctx context.Context, cursor string, limit int) ([]LedgerView, string, error) {
	return a.next.List(ctx, cursor, limit)
}

type authorizedSettingsCommands struct {
	auth Authorizer
	next SettingsCommands
}

func (a authorizedSettingsCommands) PutRule(ctx context.Context, ledgerName string, rule ledger.Rule) (ledger.Rule, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerConfigure, ledgerName); err != nil {
		return ledger.Rule{}, err
	}
	return a.next.PutRule(ctx, ledgerName, rule)
}

func (a authorizedSettingsCommands) DeleteRule(ctx context.Context, ledgerName, pattern string) error {
	if err := a.auth.Require(ctx, ledger.PermLedgerConfigure, ledgerName); err != nil {
		return err
	}
	return a.next.DeleteRule(ctx, ledgerName, pattern)
}

func (a authorizedSettingsCommands) CreateEventType(ctx context.Context, ledgerName, name string, code int) (ledger.EventType, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerConfigure, ledgerName); err != nil {
		return ledger.EventType{}, err
	}
	return a.next.CreateEventType(ctx, ledgerName, name, code)
}

type authorizedSettingsQueries struct {
	auth Authorizer
	next SettingsQueries
}

func (a authorizedSettingsQueries) Rules(ctx context.Context, ledgerName string) ([]ledger.Rule, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return nil, err
	}
	return a.next.Rules(ctx, ledgerName)
}

func (a authorizedSettingsQueries) MatchRule(ctx context.Context, ledgerName, address string) (RuleMatch, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return RuleMatch{}, err
	}
	return a.next.MatchRule(ctx, ledgerName, address)
}

func (a authorizedSettingsQueries) EventTypes(ctx context.Context, ledgerName string) ([]EventTypeView, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return nil, err
	}
	return a.next.EventTypes(ctx, ledgerName)
}
