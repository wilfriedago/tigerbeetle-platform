package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Rule decides the flags of the accounts created for the addresses it matches (ADR-0006).
type Rule struct {
	Pattern   string              `json:"pattern"`
	Limit     scalar.BalanceLimit `json:"limit"`
	History   bool                `json:"history"`
	Code      int                 `json:"code" minimum:"1" maximum:"65535"`
	BuiltIn   bool                `json:"builtIn" doc:"A default that applies without being stored."`
	Segments  int                 `json:"segments" doc:"Specificity; the most specific matching rule wins."`
	CreatedAt *time.Time          `json:"createdAt,omitempty"`
	UpdatedAt *time.Time          `json:"updatedAt,omitempty"`
}

// TransformSchema adds the examples of pattern; the example tag holds only one.
func (Rule) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Properties["pattern"].Examples = []any{"users:", "world", "*"}
	return s
}

// RuleList lists the rules of a ledger, most specific first.
type RuleList struct {
	Data []Rule `json:"data"`
}

// PutRuleRequest is the body of PUT .../rules/{pattern}.
type PutRuleRequest struct {
	Limit   scalar.BalanceLimit `json:"limit"`
	History *bool               `json:"history,omitempty" default:"false"`
	Code    int                 `json:"code" minimum:"1" maximum:"65535"`
}

// RuleCandidate is a rule considered for an address.
type RuleCandidate struct {
	Rule    Rule `json:"rule"`
	Matches bool `json:"matches"`
}

// RuleMatch is the rule that would create the account for an address, and every rule considered.
type RuleMatch struct {
	Address    scalar.Address  `json:"address"`
	Rule       Rule            `json:"rule"`
	Candidates []RuleCandidate `json:"candidates"`
	Existing   []scalar.Asset  `json:"existing" doc:"Assets this address already has accounts in; those keep their frozen flags."`
}

// EventType is an event type of a ledger, carried in the code of every transfer of its writes.
type EventType struct {
	Name         string     `json:"name" example:"card.authorized"`
	Code         int        `json:"code"`
	BuiltIn      bool       `json:"builtIn"`
	CreatedAt    *time.Time `json:"createdAt,omitempty"`
	Transactions int64      `json:"transactions" doc:"Transactions and holds recorded with this event type."`
}

// EventTypeList lists the event types of a ledger.
type EventTypeList struct {
	Data []EventType `json:"data"`
}

// CreateEventTypeRequest is the body of POST .../event-types.
type CreateEventTypeRequest struct {
	Name string `json:"name" maxLength:"63"`
	Code *int   `json:"code,omitempty" minimum:"2" maximum:"65535"`
}

// RulePath is the rule named in a path by its pattern.
type RulePath struct {
	Pattern string `path:"pattern"`
}

// patternParam documents the pattern path parameter, whose description holds backticks.
func patternParam() *huma.Param {
	minLength, maxLength := 1, 256
	return &huma.Param{
		Name: "pattern", In: "path", Required: true,
		Description: "`*`, an address such as `world`, or a prefix ending with a colon such as `users:`.",
		Schema:      &huma.Schema{Type: huma.TypeString, MinLength: &minLength, MaxLength: &maxLength},
	}
}

func registerSettings(api huma.API, h *handlers) {
	tags := []string{"settings"}
	restapi.Register(api, huma.Operation{
		OperationID: "listRules", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/rules", Tags: tags,
		Summary: "List account rules",
		Description: "The rules that decide the flags of new accounts, most specific first, including the built-in\n" +
			"`*` and `world` defaults. A rule change applies to accounts created afterwards; existing\n" +
			"accounts keep the flags stored on their row (ADR-0006).\n",
		Responses: restapi.Success(http.StatusOK, "The rules."),
	}, h.listRules)

	restapi.Register(api, huma.Operation{
		OperationID: "matchRule", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/rules/match", Tags: tags,
		Summary:     "Which rule wins for an address",
		Description: "The rule that would create the account for an address, and every rule considered.",
		Responses:   restapi.Success(http.StatusOK, "The winning rule."),
	}, h.matchRule)

	restapi.Register(api, huma.Operation{
		OperationID: "putRule", Method: http.MethodPut, Path: "/v1/ledgers/{ledger}/rules/{pattern}", Tags: tags,
		Summary:     "Create or replace a rule",
		Description: "Creates or replaces the rule for a pattern. Putting `*` or `world` replaces the built-in default.",
		Parameters:  []*huma.Param{patternParam()},
		Responses:   restapi.Success(http.StatusOK, "The rule."),
	}, h.putRule)

	restapi.Register(api, huma.Operation{
		OperationID: "deleteRule", Method: http.MethodDelete, Path: "/v1/ledgers/{ledger}/rules/{pattern}", Tags: tags,
		Summary:       "Delete a rule",
		Description:   "Deletes a stored rule. Deleting a stored `*` or `world` restores the built-in default.",
		Parameters:    []*huma.Param{patternParam()},
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The rule was deleted."),
	}, h.deleteRule)

	restapi.Register(api, huma.Operation{
		OperationID: "listEventTypes", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/event-types", Tags: tags,
		Summary:     "List event types",
		Description: "The event types of the ledger with their TigerBeetle `code` and usage.",
		Responses:   restapi.Success(http.StatusOK, "The event types."),
	}, h.listEventTypes)

	restapi.Register(api, huma.Operation{
		OperationID: "createEventType", Method: http.MethodPost, Path: "/v1/ledgers/{ledger}/event-types", Tags: tags,
		Summary: "Create an event type",
		Description: "Registers an event type. Event types are append-only: a code never changes meaning, so\n" +
			"there is no edit or delete (ADR-0007). Without a code, the next free one is assigned.\n",
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The event type."),
	}, h.createEventType)
}

func (h *handlers) listRules(ctx context.Context, in *LedgerPath) (*body[RuleList], error) {
	rules, err := h.Settings.Rules(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	out := RuleList{Data: make([]Rule, len(rules))}
	for i, r := range rules {
		out.Data[i] = ruleOf(r)
	}
	return &body[RuleList]{Body: out}, nil
}

func (h *handlers) matchRule(ctx context.Context, in *struct {
	LedgerPath
	Address scalar.Address `query:"address" required:"true"`
},
) (*body[RuleMatch], error) {
	m, err := h.Settings.MatchRule(ctx, string(in.Ledger), string(in.Address))
	if err != nil {
		return nil, err
	}
	out := RuleMatch{
		Address: scalar.Address(m.Address), Rule: ruleOf(m.Rule),
		Candidates: make([]RuleCandidate, len(m.Candidates)), Existing: assetsOf(m.Existing),
	}
	for i, c := range m.Candidates {
		out.Candidates[i] = RuleCandidate{Rule: ruleOf(c.Rule), Matches: c.Matches}
	}
	return &body[RuleMatch]{Body: out}, nil
}

func (h *handlers) putRule(ctx context.Context, in *struct {
	LedgerPath
	RulePath
	Body PutRuleRequest
},
) (*body[Rule], error) {
	if in.Body.Code < 1 || in.Body.Code > 65535 {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "code must be between 1 and 65535; TigerBeetle rejects code 0")
	}
	rule, err := h.Settings.PutRule(ctx, string(in.Ledger), ledger.Rule{
		Pattern: ledger.Pattern(in.Pattern),
		Limit:   ledger.Limit(in.Body.Limit),
		History: deref(in.Body.History),
		Code:    uint16(in.Body.Code),
	})
	if err != nil {
		return nil, err
	}
	return &body[Rule]{Body: ruleOf(rule)}, nil
}

func (h *handlers) deleteRule(ctx context.Context, in *struct {
	LedgerPath
	RulePath
},
) (*struct{}, error) {
	if err := h.Settings.DeleteRule(ctx, string(in.Ledger), in.Pattern); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}

func (h *handlers) listEventTypes(ctx context.Context, in *LedgerPath) (*body[EventTypeList], error) {
	types, err := h.Settings.EventTypes(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	out := EventTypeList{Data: make([]EventType, len(types))}
	for i, t := range types {
		out.Data[i] = EventType{Name: t.Name, Code: int(t.Code), BuiltIn: t.BuiltIn, Transactions: t.Uses}
		if !t.CreatedAt.IsZero() {
			created := t.CreatedAt
			out.Data[i].CreatedAt = &created
		}
	}
	return &body[EventTypeList]{Body: out}, nil
}

func (h *handlers) createEventType(ctx context.Context, in *struct {
	LedgerPath
	Body CreateEventTypeRequest
},
) (*body[EventType], error) {
	e, err := h.Settings.CreateEventType(ctx, string(in.Ledger), in.Body.Name, deref(in.Body.Code))
	if err != nil {
		return nil, err
	}
	created := e.CreatedAt
	return &body[EventType]{Body: EventType{Name: e.Name, Code: int(e.Code), CreatedAt: &created}}, nil
}

func ruleOf(r ledger.Rule) Rule {
	out := Rule{
		Pattern:  string(r.Pattern),
		Limit:    scalar.BalanceLimit(r.Limit),
		History:  r.History,
		Code:     int(r.Code),
		BuiltIn:  r.BuiltIn,
		Segments: r.Pattern.Segments(),
	}
	if !r.CreatedAt.IsZero() {
		created, updated := r.CreatedAt, r.UpdatedAt
		out.CreatedAt, out.UpdatedAt = &created, &updated
	}
	return out
}
