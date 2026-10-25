package app

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// Settings manages the account rules and event types of a ledger (ADR-0006, ADR-0007).
type Settings struct {
	ledgers  LedgerRepository
	rules    RuleRepository
	events   EventTypeRepository
	accounts AccountRepository
	usage    TransactionRepository
	clock    Clock
}

// NewSettings builds the use case.
func NewSettings(ledgers LedgerRepository, rules RuleRepository, events EventTypeRepository, accounts AccountRepository, usage TransactionRepository, clock Clock) *Settings {
	return &Settings{ledgers: ledgers, rules: rules, events: events, accounts: accounts, usage: usage, clock: clock}
}

func (u *Settings) requireLedger(ctx context.Context, name string) error {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return err
	}
	if _, err := u.ledgers.GetLedger(ctx, name); err != nil {
		return fmt.Errorf("get ledger %q: %w", name, err)
	}
	return nil
}

// Rules returns the effective rules, most specific first, built-in defaults included.
func (u *Settings) Rules(ctx context.Context, ledgerName string) ([]ledger.Rule, error) {
	if err := u.requireLedger(ctx, ledgerName); err != nil {
		return nil, err
	}
	stored, err := u.rules.ListRules(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("list rules of %q: %w", ledgerName, err)
	}
	return ledger.EffectiveRules(stored), nil
}

// PutRule creates or replaces the rule for a pattern. It applies to accounts created afterwards.
func (u *Settings) PutRule(ctx context.Context, ledgerName string, rule ledger.Rule) (ledger.Rule, error) {
	if err := u.requireLedger(ctx, ledgerName); err != nil {
		return ledger.Rule{}, err
	}
	if err := rule.Validate(); err != nil {
		return ledger.Rule{}, err
	}
	rule.BuiltIn = false
	now := u.clock.Now().UTC()
	rule.CreatedAt, rule.UpdatedAt = now, now
	saved, err := u.rules.PutRule(ctx, ledgerName, rule)
	if err != nil {
		return ledger.Rule{}, fmt.Errorf("put rule %q: %w", rule.Pattern, err)
	}
	return saved, nil
}

// DeleteRule removes a stored rule; deleting a stored "*" or "world" restores the default.
func (u *Settings) DeleteRule(ctx context.Context, ledgerName, pattern string) error {
	if err := u.requireLedger(ctx, ledgerName); err != nil {
		return err
	}
	p, err := ledger.ParsePattern(pattern)
	if err != nil {
		return err
	}
	if err := u.rules.DeleteRule(ctx, ledgerName, p); err != nil {
		if errors.Is(err, ledger.ErrNotFound) && ledger.IsBuiltInPattern(p) {
			return ledger.NewProblem(ledger.ErrInvalid, "built-in-rule", fmt.Sprintf("%s is a built-in default; put a rule for it to replace it", p))
		}
		return fmt.Errorf("delete rule %q: %w", p, err)
	}
	return nil
}

// RuleMatch is the answer to "which rule wins for this address".
type RuleMatch struct {
	Address    ledger.Address
	Rule       ledger.Rule
	Candidates []ledger.RuleCandidate
	// Existing lists the assets the address already has accounts in; their flags are frozen.
	Existing []string
}

// MatchRule resolves the rule that would create the account for an address.
func (u *Settings) MatchRule(ctx context.Context, ledgerName, address string) (RuleMatch, error) {
	if err := u.requireLedger(ctx, ledgerName); err != nil {
		return RuleMatch{}, err
	}
	a, err := ledger.ParseAddress(address)
	if err != nil {
		return RuleMatch{}, err
	}
	stored, err := u.rules.ListRules(ctx, ledgerName)
	if err != nil {
		return RuleMatch{}, fmt.Errorf("list rules of %q: %w", ledgerName, err)
	}
	rule, candidates := ledger.MatchRule(stored, a)
	match := RuleMatch{Address: a, Rule: rule, Candidates: candidates, Existing: []string{}}
	row, err := u.accounts.GetAddress(ctx, ledgerName, a)
	switch {
	case err == nil:
		for i := range row.Accounts {
			acc := &row.Accounts[i]
			match.Existing = append(match.Existing, acc.Asset)
		}
	case errors.Is(err, ledger.ErrNotFound):
	default:
		return RuleMatch{}, fmt.Errorf("get address %q: %w", a, err)
	}
	return match, nil
}

// EventTypes lists the registry, the built-in default first, with usage counts.
func (u *Settings) EventTypes(ctx context.Context, ledgerName string) ([]EventTypeUsage, error) {
	if err := u.requireLedger(ctx, ledgerName); err != nil {
		return nil, err
	}
	stored, err := u.events.ListEventTypes(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("list event types of %q: %w", ledgerName, err)
	}
	usage, err := u.usage.EventTypeUsage(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("count event type usage of %q: %w", ledgerName, err)
	}
	out := make([]EventTypeUsage, 0, len(stored)+1)
	out = append(out, EventTypeUsage{EventType: ledger.DefaultEventType, Uses: usage[ledger.DefaultEventType.Name]})
	for _, e := range stored {
		e.Uses = usage[e.Name]
		out = append(out, e)
	}
	slices.SortStableFunc(out[1:], func(a, b EventTypeUsage) int { return int(a.Code) - int(b.Code) })
	return out, nil
}

// CreateEventType registers an event type. A zero code takes the next free one.
func (u *Settings) CreateEventType(ctx context.Context, ledgerName, name string, code int) (ledger.EventType, error) {
	if err := u.requireLedger(ctx, ledgerName); err != nil {
		return ledger.EventType{}, err
	}
	if err := ledger.ValidateEventTypeName(name); err != nil {
		return ledger.EventType{}, err
	}
	if name == ledger.DefaultEventType.Name {
		return ledger.EventType{}, ledger.NewProblem(ledger.ErrAlreadyExists, "already-exists", "default is the built-in event type")
	}
	if code != 0 {
		if err := ledger.ValidateEventTypeCode(code); err != nil {
			return ledger.EventType{}, err
		}
	}
	created, err := u.events.CreateEventType(ctx, ledgerName, name, uint16(code))
	if err != nil {
		return ledger.EventType{}, fmt.Errorf("create event type %q: %w", name, err)
	}
	return created, nil
}

// resolveEventType returns the named event type, or the default for an empty name.
func resolveEventType(ctx context.Context, events EventTypeRepository, ledgerName, name string) (ledger.EventType, error) {
	if name == "" || name == ledger.DefaultEventType.Name {
		return ledger.DefaultEventType, nil
	}
	e, err := events.GetEventType(ctx, ledgerName, name)
	if errors.Is(err, ledger.ErrNotFound) {
		return ledger.EventType{}, ledger.NewProblem(ledger.ErrInvalid, "unknown-event-type",
			fmt.Sprintf("event type %q is not registered in ledger %s; create it under Settings, Event types", name, ledgerName))
	}
	if err != nil {
		return ledger.EventType{}, fmt.Errorf("get event type %q: %w", name, err)
	}
	return e, nil
}
