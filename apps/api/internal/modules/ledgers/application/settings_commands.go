package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/domain"
)

type settingsCommands struct {
	directory *Directory
	rules     domain.RuleRepository
	events    domain.EventTypeRepository
	clock     ledger.Clock
}

func (u *settingsCommands) PutRule(ctx context.Context, ledgerName string, rule ledger.Rule) (ledger.Rule, error) {
	if err := u.directory.Require(ctx, ledgerName); err != nil {
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

func (u *settingsCommands) DeleteRule(ctx context.Context, ledgerName, pattern string) error {
	if err := u.directory.Require(ctx, ledgerName); err != nil {
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

func (u *settingsCommands) CreateEventType(ctx context.Context, ledgerName, name string, code int) (ledger.EventType, error) {
	if err := u.directory.Require(ctx, ledgerName); err != nil {
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
