// Package v1 is the ledgers module's broker RPC interface (ADR-0021, plan section 11.2): one
// method per REST operation, v1.<operationId>, with the REST parameter names, body and answer.
// The same routes serve AMQP and NATS.
package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

type (
	ledgerParams struct {
		Ledger string `json:"ledger"`
	}
	pageParams struct {
		Cursor string `json:"cursor"`
		Limit  int    `json:"limit"`
	}
	addressParams struct {
		Ledger  string `json:"ledger"`
		Address string `json:"address"`
	}
	ruleParams struct {
		Ledger  string `json:"ledger"`
		Pattern string `json:"pattern"`
	}
)

// Register adds the ledgers methods.
func Register(r *rpc.Router, app application.Application) {
	r.Handle("v1.listLedgers", rpc.Bind(func(ctx context.Context, p pageParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		views, next, err := app.LedgerQueries.List(ctx, p.Cursor, p.Limit)
		if err != nil {
			return nil, err
		}
		out := dto.LedgerList{Data: make([]dto.Ledger, len(views)), Next: optional(next)}
		for i, v := range views {
			out.Data[i] = dto.LedgerFrom(v)
		}
		return out, nil
	}))
	r.Handle("v1.getLedger", rpc.Bind(func(ctx context.Context, p ledgerParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		v, err := app.LedgerQueries.Get(ctx, p.Ledger)
		if err != nil {
			return nil, err
		}
		return dto.LedgerFrom(v), nil
	}))
	r.Handle("v1.createLedger", rpc.Bind(func(ctx context.Context, p ledgerParams, b dto.CreateLedgerRequest, _ rpc.Request) (any, error) {
		v, err := app.LedgerCommands.Create(ctx, p.Ledger, b.Metadata.Map())
		if err != nil {
			return nil, err
		}
		return dto.LedgerFrom(v), nil
	}))
	r.Handle("v1.replaceLedgerMetadata", rpc.Bind(func(ctx context.Context, p ledgerParams, b scalar.Metadata, _ rpc.Request) (any, error) {
		v, err := app.LedgerCommands.ReplaceMetadata(ctx, p.Ledger, b.Map())
		if err != nil {
			return nil, err
		}
		return dto.LedgerFrom(v), nil
	}))

	r.Handle("v1.listRules", rpc.Bind(func(ctx context.Context, p ledgerParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		rules, err := app.SettingsQueries.Rules(ctx, p.Ledger)
		if err != nil {
			return nil, err
		}
		out := dto.RuleList{Data: make([]dto.Rule, len(rules))}
		for i := range rules {
			out.Data[i] = dto.RuleFrom(rules[i])
		}
		return out, nil
	}))
	r.Handle("v1.matchRule", rpc.Bind(func(ctx context.Context, p addressParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		m, err := app.SettingsQueries.MatchRule(ctx, p.Ledger, p.Address)
		if err != nil {
			return nil, err
		}
		return dto.RuleMatchFrom(m), nil
	}))
	r.Handle("v1.putRule", rpc.Bind(func(ctx context.Context, p ruleParams, b dto.PutRuleRequest, _ rpc.Request) (any, error) {
		if b.Code < 1 || b.Code > 65535 {
			return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "code must be between 1 and 65535; TigerBeetle rejects code 0")
		}
		rule, err := app.SettingsCommands.PutRule(ctx, p.Ledger, ledger.Rule{
			Pattern: ledger.Pattern(p.Pattern),
			Limit:   ledger.Limit(b.Limit),
			History: b.History != nil && *b.History,
			Code:    uint16(b.Code),
		})
		if err != nil {
			return nil, err
		}
		return dto.RuleFrom(rule), nil
	}))
	r.Handle("v1.deleteRule", rpc.Bind(func(ctx context.Context, p ruleParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		return nil, app.SettingsCommands.DeleteRule(ctx, p.Ledger, p.Pattern)
	}))
	r.Handle("v1.listEventTypes", rpc.Bind(func(ctx context.Context, p ledgerParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		types, err := app.SettingsQueries.EventTypes(ctx, p.Ledger)
		if err != nil {
			return nil, err
		}
		out := dto.EventTypeList{Data: make([]dto.EventType, len(types))}
		for i, t := range types {
			out.Data[i] = dto.EventTypeFrom(t)
		}
		return out, nil
	}))
	r.Handle("v1.createEventType", rpc.Bind(func(ctx context.Context, p ledgerParams, b dto.CreateEventTypeRequest, _ rpc.Request) (any, error) {
		code := 0
		if b.Code != nil {
			code = *b.Code
		}
		e, err := app.SettingsCommands.CreateEventType(ctx, p.Ledger, b.Name, code)
		if err != nil {
			return nil, err
		}
		created := e.CreatedAt
		return dto.EventType{Name: e.Name, Code: int(e.Code), CreatedAt: &created}, nil
	}))
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
