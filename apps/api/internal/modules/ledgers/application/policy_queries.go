package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/domain"
)

// PolicyQueries answers bookkeeping's AccountPolicies and EventTypes ports: which rule creates the
// account of an address (ADR-0006), and which code an event type name carries (ADR-0007). They read
// only the ledgers module's tables.
type PolicyQueries struct {
	rules  domain.RuleRepository
	events domain.EventTypeRepository
}

// NewPolicyQueries builds the queries. Bootstrap builds them before bookkeeping, which needs them.
func NewPolicyQueries(rules domain.RuleRepository, events domain.EventTypeRepository) *PolicyQueries {
	return &PolicyQueries{rules: rules, events: events}
}

// PoliciesFor returns the winning rule of each address, built-in defaults included.
func (q *PolicyQueries) PoliciesFor(ctx context.Context, ledgerName string, addresses []ledger.Address) (map[ledger.Address]ledger.Rule, error) {
	stored, err := q.rules.ListRules(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("list rules of %q: %w", ledgerName, err)
	}
	out := make(map[ledger.Address]ledger.Rule, len(addresses))
	for _, a := range addresses {
		out[a], _ = ledger.MatchRule(stored, a)
	}
	return out, nil
}

// Resolve returns the named event type, or the default for an empty name. An unknown name is an
// invalid request that says where to create it.
func (q *PolicyQueries) Resolve(ctx context.Context, ledgerName, name string) (ledger.EventType, error) {
	if name == "" || name == ledger.DefaultEventType.Name {
		return ledger.DefaultEventType, nil
	}
	e, err := q.events.GetEventType(ctx, ledgerName, name)
	if errors.Is(err, ledger.ErrNotFound) {
		return ledger.EventType{}, ledger.NewProblem(ledger.ErrInvalid, "unknown-event-type",
			fmt.Sprintf("event type %q is not registered in ledger %s; create it under Settings, Event types", name, ledgerName))
	}
	if err != nil {
		return ledger.EventType{}, fmt.Errorf("get event type %q: %w", name, err)
	}
	return e, nil
}
