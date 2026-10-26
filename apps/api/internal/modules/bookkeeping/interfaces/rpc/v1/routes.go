// Package v1 is the bookkeeping module's broker RPC interface (ADR-0021, plan section 11.2): one
// method per REST operation, v1.<operationId>, with the REST parameter names, body and answer.
// The same routes serve AMQP and NATS. A money write takes its key from the idempotency-key
// header and reaches the same use case as REST, so a key replays across transports.
package v1

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

type (
	ledgerParams struct {
		Ledger string `json:"ledger"`
	}
	accountParams struct {
		Ledger  string `json:"ledger"`
		Address string `json:"address"`
	}
	resourceParams struct {
		Ledger string `json:"ledger"`
		ID     string `json:"id"`
	}
	listAccountsParams struct {
		Ledger string `json:"ledger"`
		Prefix string `json:"prefix"`
		Search string `json:"search"`
		Asset  string `json:"asset"`
		Cursor string `json:"cursor"`
		Limit  int    `json:"limit"`
	}
	historyParams struct {
		Ledger  string `json:"ledger"`
		Address string `json:"address"`
		Asset   string `json:"asset"`
		Limit   int    `json:"limit"`
	}
	segmentParams struct {
		Ledger string `json:"ledger"`
		Prefix string `json:"prefix"`
	}
	searchParams struct {
		Ledger string `json:"ledger"`
		Q      string `json:"q"`
	}
	listTransactionsParams struct {
		Ledger    string     `json:"ledger"`
		Address   string     `json:"address"`
		Asset     string     `json:"asset"`
		Reference string     `json:"reference"`
		EventType string     `json:"eventType"`
		After     *time.Time `json:"after"`
		Before    *time.Time `json:"before"`
		Cursor    string     `json:"cursor"`
		Limit     int        `json:"limit"`
	}
	listHoldsParams struct {
		Ledger  string `json:"ledger"`
		Status  string `json:"status"`
		Address string `json:"address"`
		Cursor  string `json:"cursor"`
		Limit   int    `json:"limit"`
	}
	listRequestsParams struct {
		Ledger    string `json:"ledger"`
		Status    string `json:"status"`
		Operation string `json:"operation"`
		Search    string `json:"search"`
		Cursor    string `json:"cursor"`
		Limit     int    `json:"limit"`
	}
	requestParams struct {
		Ledger string `json:"ledger"`
		Key    string `json:"key"`
	}
)

// Register adds the bookkeeping methods.
func Register(r *rpc.Router, app application.Application) {
	registerAccounts(r, app)
	registerOverview(r, app)
	registerTransactions(r, app)
	registerHolds(r, app)
	registerRequests(r, app)
}

func registerAccounts(r *rpc.Router, app application.Application) {
	r.Handle("v1.listAccounts", rpc.Bind(func(ctx context.Context, p listAccountsParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		views, next, err := app.AccountQueries.List(ctx, application.ListAccountsInput{
			Ledger: p.Ledger, Prefix: p.Prefix, Search: p.Search, Asset: p.Asset, Cursor: p.Cursor, Limit: p.Limit,
		})
		if err != nil {
			return nil, err
		}
		out := dto.AccountList{Data: make([]dto.AccountSummary, len(views)), Next: optional(next)}
		for i := range views {
			out.Data[i] = dto.AccountSummaryFrom(&views[i])
		}
		return out, nil
	}))
	r.Handle("v1.getAccount", rpc.Bind(func(ctx context.Context, p accountParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		v, err := app.AccountQueries.Get(ctx, p.Ledger, p.Address)
		if err != nil {
			return nil, err
		}
		return dto.AccountFrom(&v), nil
	}))
	r.Handle("v1.replaceAccountMetadata", rpc.Bind(func(ctx context.Context, p accountParams, b scalar.Metadata, _ rpc.Request) (any, error) {
		v, err := app.AccountCommands.ReplaceMetadata(ctx, p.Ledger, p.Address, b.Map())
		if err != nil {
			return nil, err
		}
		return dto.AccountFrom(&v), nil
	}))
	r.Handle("v1.getAccountHistory", rpc.Bind(func(ctx context.Context, p historyParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		points, err := app.AccountQueries.History(ctx, p.Ledger, p.Address, p.Asset, p.Limit)
		if err != nil {
			return nil, err
		}
		return dto.BalanceHistoryFrom(scalar.Address(p.Address), scalar.Asset(p.Asset), points), nil
	}))
	r.Handle("v1.aggregateBalances", rpc.Bind(func(ctx context.Context, p accountParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		agg, err := app.AccountQueries.AggregateBalances(ctx, p.Ledger, p.Address)
		if err != nil {
			return nil, err
		}
		return dto.AggregateBalancesFrom(&agg), nil
	}))
	r.Handle("v1.listSegments", rpc.Bind(func(ctx context.Context, p segmentParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		prefix, rows, err := app.AccountQueries.Segments(ctx, p.Ledger, p.Prefix)
		if err != nil {
			return nil, err
		}
		out := dto.SegmentList{Prefix: optional(prefix), Data: make([]dto.Segment, len(rows))}
		for i := range rows {
			out.Data[i] = dto.SegmentFrom(&rows[i])
		}
		return out, nil
	}))
}

func registerOverview(r *rpc.Router, app application.Application) {
	r.Handle("v1.listAssets", rpc.Bind(func(ctx context.Context, p ledgerParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		assets, err := app.AccountQueries.Assets(ctx, p.Ledger)
		if err != nil {
			return nil, err
		}
		return dto.AssetList{Data: dto.AssetSummariesFrom(assets)}, nil
	}))
	r.Handle("v1.getLedgerStats", rpc.Bind(func(ctx context.Context, p ledgerParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		st, err := app.OverviewQueries.Stats(ctx, p.Ledger)
		if err != nil {
			return nil, err
		}
		return dto.LedgerStatsFrom(&st), nil
	}))
	r.Handle("v1.search", rpc.Bind(func(ctx context.Context, p searchParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		res, err := app.OverviewQueries.Search(ctx, p.Ledger, p.Q)
		if err != nil {
			return nil, err
		}
		return dto.SearchResultsFrom(p.Q, &res), nil
	}))
	r.Handle("v1.getReconcilerStatus", rpc.Bind(func(ctx context.Context, _ struct{}, _ rpc.NoBody, _ rpc.Request) (any, error) {
		st, err := app.ReconcilerQueries.Status(ctx)
		if err != nil {
			return nil, err
		}
		return dto.ReconcilerStatusFrom(&st), nil
	}))
}

func registerTransactions(r *rpc.Router, app application.Application) {
	r.Handle("v1.listTransactions", rpc.Bind(func(ctx context.Context, p listTransactionsParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		views, next, err := app.TransactionQueries.List(ctx, application.ListTransactionsInput{
			Ledger: p.Ledger, Address: p.Address, Asset: p.Asset, Reference: p.Reference, EventType: p.EventType,
			After: p.After, Before: p.Before, Cursor: p.Cursor, Limit: p.Limit,
		})
		if err != nil {
			return nil, err
		}
		out := dto.TransactionList{Data: make([]dto.Transaction, len(views)), Next: optional(next)}
		for i := range views {
			out.Data[i] = dto.TransactionFrom(&views[i])
		}
		return out, nil
	}))
	r.Handle("v1.getTransaction", rpc.Bind(func(ctx context.Context, p resourceParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		v, err := app.TransactionQueries.Get(ctx, p.Ledger, p.ID)
		if err != nil {
			return nil, err
		}
		return dto.TransactionFrom(&v), nil
	}))
	r.Handle("v1.createTransaction", rpc.Bind(func(ctx context.Context, p ledgerParams, b dto.CreateTransactionRequest, req rpc.Request) (any, error) {
		postings, err := dto.ParsePostings(b.Postings)
		if err != nil {
			return nil, err
		}
		v, err := app.TransactionCommands.Create(ctx, application.CreateTransactionInput{
			Ledger:       p.Ledger,
			Key:          req.IdempotencyKey,
			RequestID:    requestid.From(ctx),
			Postings:     postings,
			Reference:    deref(b.Reference),
			EventType:    deref(b.EventType),
			BusinessTime: b.BusinessTime,
			Metadata:     b.Metadata.Map(),
		})
		if err != nil {
			return nil, err
		}
		return dto.TransactionFrom(&v), nil
	}))
	r.Handle("v1.replaceTransactionMetadata", rpc.Bind(func(ctx context.Context, p resourceParams, b scalar.Metadata, _ rpc.Request) (any, error) {
		v, err := app.TransactionCommands.ReplaceMetadata(ctx, p.Ledger, p.ID, b.Map())
		if err != nil {
			return nil, err
		}
		return dto.TransactionFrom(&v), nil
	}))
	r.Handle("v1.revertTransaction", rpc.Bind(func(ctx context.Context, p resourceParams, b dto.RevertTransactionRequest, req rpc.Request) (any, error) {
		v, err := app.TransactionCommands.Revert(ctx, application.RevertTransactionInput{
			Ledger:    p.Ledger,
			Key:       req.IdempotencyKey,
			RequestID: requestid.From(ctx),
			ID:        p.ID,
			Reference: deref(b.Reference),
			Metadata:  b.Metadata.Map(),
		})
		if err != nil {
			return nil, err
		}
		return dto.TransactionFrom(&v), nil
	}))
}

func registerHolds(r *rpc.Router, app application.Application) {
	r.Handle("v1.listHolds", rpc.Bind(func(ctx context.Context, p listHoldsParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		views, next, counts, err := app.HoldQueries.List(ctx, application.ListHoldsInput{
			Ledger: p.Ledger, Status: p.Status, Address: p.Address, Cursor: p.Cursor, Limit: p.Limit,
		})
		if err != nil {
			return nil, err
		}
		out := dto.HoldList{Data: make([]dto.Hold, len(views)), Next: optional(next), Counts: dto.HoldStatsFrom(counts, nil)}
		for i := range views {
			out.Data[i] = dto.HoldFrom(&views[i])
		}
		return out, nil
	}))
	r.Handle("v1.getHold", rpc.Bind(func(ctx context.Context, p resourceParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		v, err := app.HoldQueries.Get(ctx, p.Ledger, p.ID)
		if err != nil {
			return nil, err
		}
		return dto.HoldFrom(&v), nil
	}))
	r.Handle("v1.createHold", rpc.Bind(func(ctx context.Context, p ledgerParams, b dto.CreateHoldRequest, req rpc.Request) (any, error) {
		posting, err := dto.Posting{Source: b.Source, Destination: b.Destination, Asset: b.Asset, Amount: b.Amount}.Parse()
		if err != nil {
			return nil, err
		}
		timeout := deref(b.TimeoutSeconds)
		if timeout < 0 || timeout > domain.MaxHoldTimeout {
			return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "timeoutSeconds must be between 0 and 4294967295")
		}
		v, err := app.HoldCommands.Create(ctx, application.CreateHoldInput{
			Ledger:         p.Ledger,
			Key:            req.IdempotencyKey,
			RequestID:      requestid.From(ctx),
			Posting:        posting,
			TimeoutSeconds: uint32(timeout),
			Reference:      deref(b.Reference),
			EventType:      deref(b.EventType),
			Metadata:       b.Metadata.Map(),
		})
		if err != nil {
			return nil, err
		}
		return dto.HoldFrom(&v), nil
	}))
	r.Handle("v1.captureHold", rpc.Bind(func(ctx context.Context, p resourceParams, b dto.CaptureHoldRequest, req rpc.Request) (any, error) {
		capture := application.CaptureHoldInput{Ledger: p.Ledger, Key: req.IdempotencyKey, RequestID: requestid.From(ctx), ID: p.ID}
		if b.Amount != nil {
			a, err := ledger.ParseAmount(string(*b.Amount))
			if err != nil {
				return nil, err
			}
			capture.Amount = &a
		}
		v, err := app.HoldCommands.Capture(ctx, capture)
		if err != nil {
			return nil, err
		}
		return dto.HoldFrom(&v), nil
	}))
	r.Handle("v1.voidHold", rpc.Bind(func(ctx context.Context, p resourceParams, _ rpc.NoBody, req rpc.Request) (any, error) {
		v, err := app.HoldCommands.Void(ctx, application.VoidHoldInput{
			Ledger: p.Ledger, Key: req.IdempotencyKey, RequestID: requestid.From(ctx), ID: p.ID,
		})
		if err != nil {
			return nil, err
		}
		return dto.HoldFrom(&v), nil
	}))
}

func registerRequests(r *rpc.Router, app application.Application) {
	r.Handle("v1.listWriteRequests", rpc.Bind(func(ctx context.Context, p listRequestsParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		rows, next, err := app.RequestQueries.List(ctx, application.ListRequestsInput{
			Ledger: p.Ledger, Status: p.Status, Operation: p.Operation, Search: p.Search, Cursor: p.Cursor, Limit: p.Limit,
		})
		if err != nil {
			return nil, err
		}
		out := dto.WriteRequestList{Data: make([]dto.WriteRequest, len(rows)), Next: optional(next)}
		for i := range rows {
			out.Data[i] = dto.WriteRequestFrom(&rows[i])
		}
		return out, nil
	}))
	r.Handle("v1.getWriteRequest", rpc.Bind(func(ctx context.Context, p requestParams, _ rpc.NoBody, _ rpc.Request) (any, error) {
		rec, err := app.RequestQueries.Get(ctx, p.Ledger, p.Key)
		if err != nil {
			return nil, err
		}
		return dto.WriteRequestDetailFrom(&rec), nil
	}))
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
