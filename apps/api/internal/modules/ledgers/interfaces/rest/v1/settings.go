package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

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

func (h *handlers) listRules(ctx context.Context, in *restapi.LedgerPath) (*restapi.Body[dto.RuleList], error) {
	rules, err := h.SettingsQueries.Rules(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	out := dto.RuleList{Data: make([]dto.Rule, len(rules))}
	for i, r := range rules {
		out.Data[i] = dto.RuleFrom(r)
	}
	return &restapi.Body[dto.RuleList]{Body: out}, nil
}

func (h *handlers) matchRule(ctx context.Context, in *struct {
	restapi.LedgerPath
	Address scalar.Address `query:"address" required:"true"`
},
) (*restapi.Body[dto.RuleMatch], error) {
	m, err := h.SettingsQueries.MatchRule(ctx, string(in.Ledger), string(in.Address))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.RuleMatch]{Body: dto.RuleMatchFrom(m)}, nil
}

func (h *handlers) putRule(ctx context.Context, in *struct {
	restapi.LedgerPath
	RulePath
	Body dto.PutRuleRequest
},
) (*restapi.Body[dto.Rule], error) {
	if in.Body.Code < 1 || in.Body.Code > 65535 {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "code must be between 1 and 65535; TigerBeetle rejects code 0")
	}
	history := in.Body.History != nil && *in.Body.History
	rule, err := h.SettingsCommands.PutRule(ctx, string(in.Ledger), ledger.Rule{
		Pattern: ledger.Pattern(in.Pattern),
		Limit:   ledger.Limit(in.Body.Limit),
		History: history,
		Code:    uint16(in.Body.Code),
	})
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Rule]{Body: dto.RuleFrom(rule)}, nil
}

func (h *handlers) deleteRule(ctx context.Context, in *struct {
	restapi.LedgerPath
	RulePath
},
) (*struct{}, error) {
	if err := h.SettingsCommands.DeleteRule(ctx, string(in.Ledger), in.Pattern); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}

func (h *handlers) listEventTypes(ctx context.Context, in *restapi.LedgerPath) (*restapi.Body[dto.EventTypeList], error) {
	types, err := h.SettingsQueries.EventTypes(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	out := dto.EventTypeList{Data: make([]dto.EventType, len(types))}
	for i, t := range types {
		out.Data[i] = dto.EventTypeFrom(t)
	}
	return &restapi.Body[dto.EventTypeList]{Body: out}, nil
}

func (h *handlers) createEventType(ctx context.Context, in *struct {
	restapi.LedgerPath
	Body dto.CreateEventTypeRequest
},
) (*restapi.Body[dto.EventType], error) {
	code := 0
	if in.Body.Code != nil {
		code = *in.Body.Code
	}
	e, err := h.SettingsCommands.CreateEventType(ctx, string(in.Ledger), in.Body.Name, code)
	if err != nil {
		return nil, err
	}
	created := e.CreatedAt
	return &restapi.Body[dto.EventType]{Body: dto.EventType{Name: e.Name, Code: int(e.Code), CreatedAt: &created}}, nil
}
