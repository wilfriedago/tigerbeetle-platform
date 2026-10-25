package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/domain"
)

type settingsQueries struct {
	directory *Directory
	rules     domain.RuleRepository
	events    domain.EventTypeRepository
	usage     EventTypeUsage
	addresses AddressAssets
}

func (u *settingsQueries) Rules(ctx context.Context, ledgerName string) ([]ledger.Rule, error) {
	if err := u.directory.Require(ctx, ledgerName); err != nil {
		return nil, err
	}
	stored, err := u.rules.ListRules(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("list rules of %q: %w", ledgerName, err)
	}
	return ledger.EffectiveRules(stored), nil
}

func (u *settingsQueries) MatchRule(ctx context.Context, ledgerName, address string) (RuleMatch, error) {
	if err := u.directory.Require(ctx, ledgerName); err != nil {
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
	existing, err := u.addresses.AddressAssets(ctx, ledgerName, a)
	if err != nil {
		return RuleMatch{}, fmt.Errorf("assets of %q: %w", a, err)
	}
	if existing == nil {
		existing = []string{}
	}
	return RuleMatch{Address: a, Rule: rule, Candidates: candidates, Existing: existing}, nil
}

func (u *settingsQueries) EventTypes(ctx context.Context, ledgerName string) ([]EventTypeView, error) {
	if err := u.directory.Require(ctx, ledgerName); err != nil {
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
	out := make([]EventTypeView, 0, len(stored)+1)
	out = append(out, EventTypeView{EventType: ledger.DefaultEventType, Uses: usage[ledger.DefaultEventType.Name]})
	for _, e := range stored {
		out = append(out, EventTypeView{EventType: e, Uses: usage[e.Name]})
	}
	slices.SortStableFunc(out[1:], func(a, b EventTypeView) int { return int(a.Code) - int(b.Code) })
	return out, nil
}
