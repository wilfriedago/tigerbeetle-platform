package httpapi

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// ListRules returns the effective rules of a ledger.
func (s *server) ListRules(ctx context.Context, req gen.ListRulesRequestObject) (gen.ListRulesResponseObject, error) {
	rules, err := s.settings.Rules(ctx, req.Ledger)
	if err != nil {
		return nil, err
	}
	data := make([]gen.Rule, len(rules))
	for i, r := range rules {
		data[i] = ruleOf(r)
	}
	return gen.ListRules200JSONResponse{Data: data}, nil
}

// MatchRule resolves the rule for an address.
func (s *server) MatchRule(ctx context.Context, req gen.MatchRuleRequestObject) (gen.MatchRuleResponseObject, error) {
	m, err := s.settings.MatchRule(ctx, req.Ledger, req.Params.Address)
	if err != nil {
		return nil, err
	}
	candidates := make([]gen.RuleCandidate, len(m.Candidates))
	for i, c := range m.Candidates {
		candidates[i] = gen.RuleCandidate{Rule: ruleOf(c.Rule), Matches: c.Matches}
	}
	return gen.MatchRule200JSONResponse{Address: string(m.Address), Rule: ruleOf(m.Rule), Candidates: candidates, Existing: assetsOf(m.Existing)}, nil
}

// PutRule creates or replaces a rule.
func (s *server) PutRule(ctx context.Context, req gen.PutRuleRequestObject) (gen.PutRuleResponseObject, error) {
	if req.Body == nil {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "a JSON body with limit and code is required")
	}
	if req.Body.Code < 1 || req.Body.Code > 65535 {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "code must be between 1 and 65535; TigerBeetle rejects code 0")
	}
	rule, err := s.settings.PutRule(ctx, req.Ledger, ledger.Rule{
		Pattern: ledger.Pattern(req.Pattern),
		Limit:   ledger.Limit(req.Body.Limit),
		History: deref(req.Body.History),
		Code:    uint16(req.Body.Code),
	})
	if err != nil {
		return nil, err
	}
	return gen.PutRule200JSONResponse(ruleOf(rule)), nil
}

// DeleteRule deletes a stored rule.
func (s *server) DeleteRule(ctx context.Context, req gen.DeleteRuleRequestObject) (gen.DeleteRuleResponseObject, error) {
	if err := s.settings.DeleteRule(ctx, req.Ledger, req.Pattern); err != nil {
		return nil, err
	}
	return gen.DeleteRule204Response{}, nil
}

// ListEventTypes returns the event types of a ledger.
func (s *server) ListEventTypes(ctx context.Context, req gen.ListEventTypesRequestObject) (gen.ListEventTypesResponseObject, error) {
	types, err := s.settings.EventTypes(ctx, req.Ledger)
	if err != nil {
		return nil, err
	}
	data := make([]gen.EventType, len(types))
	for i, t := range types {
		data[i] = gen.EventType{Name: t.Name, Code: int(t.Code), BuiltIn: t.BuiltIn, Transactions: t.Uses}
		if !t.CreatedAt.IsZero() {
			created := t.CreatedAt
			data[i].CreatedAt = &created
		}
	}
	return gen.ListEventTypes200JSONResponse{Data: data}, nil
}

// CreateEventType registers an event type.
func (s *server) CreateEventType(ctx context.Context, req gen.CreateEventTypeRequestObject) (gen.CreateEventTypeResponseObject, error) {
	if req.Body == nil {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "a JSON body with a name is required")
	}
	e, err := s.settings.CreateEventType(ctx, req.Ledger, req.Body.Name, deref(req.Body.Code))
	if err != nil {
		return nil, err
	}
	created := e.CreatedAt
	return gen.CreateEventType201JSONResponse{Name: e.Name, Code: int(e.Code), BuiltIn: false, CreatedAt: &created, Transactions: 0}, nil
}
