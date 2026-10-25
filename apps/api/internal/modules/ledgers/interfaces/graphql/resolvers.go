package graphql

import (
	"context"
	"errors"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/graphqlschema"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// LedgerFrom renders a ledger for GraphQL.
func LedgerFrom(v application.LedgerView) *Ledger {
	return &Ledger{
		Name:         scalar.LedgerName(v.Name),
		Metadata:     scalar.MetadataOf(v.Metadata),
		CreatedAt:    v.CreatedAt,
		AssetCodes:   scalar.AssetsOf(v.Assets),
		AccountCount: v.Accounts,
	}
}

// Queries holds the ledgers query resolvers.
type Queries struct {
	ledgers application.LedgerQueries
}

// NewQueries builds the query resolvers.
func NewQueries(app application.Application) *Queries {
	return &Queries{ledgers: app.LedgerQueries}
}

// Ledger resolves Query.ledger: null when no ledger has that name.
func (q *Queries) Ledger(ctx context.Context, name string) (*Ledger, error) {
	v, err := q.ledgers.Get(ctx, name)
	if errors.Is(err, ledger.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return LedgerFrom(v), nil
}

// Ledgers resolves Query.ledgers.
func (q *Queries) Ledgers(ctx context.Context, first *int, after *string) (*LedgerConnection, error) {
	views, next, err := q.ledgers.List(ctx, deref(after), deref(first))
	if err != nil {
		return nil, err
	}
	out := &LedgerConnection{Nodes: make([]Ledger, len(views)), PageInfo: graphqlschema.PageInfoOf(next)}
	for i, v := range views {
		out.Nodes[i] = *LedgerFrom(v)
	}
	return out, nil
}

// Mutations holds the ledgers mutation resolvers.
type Mutations struct {
	ledgers  application.LedgerCommands
	settings application.SettingsCommands
}

// NewMutations builds the mutation resolvers.
func NewMutations(app application.Application) *Mutations {
	return &Mutations{ledgers: app.LedgerCommands, settings: app.SettingsCommands}
}

// CreateLedger resolves Mutation.createLedger.
func (m *Mutations) CreateLedger(ctx context.Context, input CreateLedgerInput) (*Ledger, error) {
	var metadata map[string]string
	if input.Metadata != nil {
		metadata = *input.Metadata
	}
	v, err := m.ledgers.Create(ctx, string(input.Name), metadata)
	if err != nil {
		return nil, err
	}
	return LedgerFrom(v), nil
}

// ReplaceLedgerMetadata resolves Mutation.replaceLedgerMetadata.
func (m *Mutations) ReplaceLedgerMetadata(ctx context.Context, input ReplaceLedgerMetadataInput) (*Ledger, error) {
	v, err := m.ledgers.ReplaceMetadata(ctx, string(input.Ledger), input.Metadata.Map())
	if err != nil {
		return nil, err
	}
	return LedgerFrom(v), nil
}

// PutRule resolves Mutation.putRule.
func (m *Mutations) PutRule(ctx context.Context, input PutRuleInput) (*dto.Rule, error) {
	if input.Code < 1 || input.Code > 65535 {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "code must be between 1 and 65535; TigerBeetle rejects code 0")
	}
	rule, err := m.settings.PutRule(ctx, string(input.Ledger), ledger.Rule{
		Pattern: ledger.Pattern(input.Pattern),
		Limit:   ledger.Limit(input.Limit),
		History: deref(input.History),
		Code:    uint16(input.Code),
	})
	if err != nil {
		return nil, err
	}
	out := dto.RuleFrom(rule)
	return &out, nil
}

// DeleteRule resolves Mutation.deleteRule.
func (m *Mutations) DeleteRule(ctx context.Context, input DeleteRuleInput) (bool, error) {
	if err := m.settings.DeleteRule(ctx, string(input.Ledger), input.Pattern); err != nil {
		return false, err
	}
	return true, nil
}

// CreateEventType resolves Mutation.createEventType.
func (m *Mutations) CreateEventType(ctx context.Context, input CreateEventTypeInput) (*dto.EventType, error) {
	e, err := m.settings.CreateEventType(ctx, string(input.Ledger), input.Name, deref(input.Code))
	if err != nil {
		return nil, err
	}
	created := e.CreatedAt
	return &dto.EventType{Name: e.Name, Code: int(e.Code), CreatedAt: &created}, nil
}

// LedgerFields holds the resolvers of the fields this module adds to Ledger.
type LedgerFields struct {
	settings application.SettingsQueries
}

// NewLedgerFields builds the field resolvers.
func NewLedgerFields(app application.Application) *LedgerFields {
	return &LedgerFields{settings: app.SettingsQueries}
}

// Rules resolves Ledger.rules.
func (f *LedgerFields) Rules(ctx context.Context, obj *Ledger) ([]dto.Rule, error) {
	rules, err := f.settings.Rules(ctx, string(obj.Name))
	if err != nil {
		return nil, err
	}
	out := make([]dto.Rule, len(rules))
	for i, r := range rules {
		out[i] = dto.RuleFrom(r)
	}
	return out, nil
}

// RuleMatch resolves Ledger.ruleMatch.
func (f *LedgerFields) RuleMatch(ctx context.Context, obj *Ledger, address string) (*dto.RuleMatch, error) {
	m, err := f.settings.MatchRule(ctx, string(obj.Name), address)
	if err != nil {
		return nil, err
	}
	out := dto.RuleMatchFrom(m)
	return &out, nil
}

// EventTypes resolves Ledger.eventTypes.
func (f *LedgerFields) EventTypes(ctx context.Context, obj *Ledger) ([]dto.EventType, error) {
	types, err := f.settings.EventTypes(ctx, string(obj.Name))
	if err != nil {
		return nil, err
	}
	out := make([]dto.EventType, len(types))
	for i, t := range types {
		out[i] = dto.EventTypeFrom(t)
	}
	return out, nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
