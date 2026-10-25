package httpapi

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
)

// ListAccounts returns a page of accounts with their balances.
func (s *server) ListAccounts(ctx context.Context, req gen.ListAccountsRequestObject) (gen.ListAccountsResponseObject, error) {
	p := req.Params
	views, next, err := s.accounts.List(ctx, req.Ledger, deref(p.Prefix), deref(p.Search), deref(p.Asset), deref(p.Cursor), deref(p.Limit))
	if err != nil {
		return nil, err
	}
	data := make([]gen.AccountSummary, len(views))
	for i, v := range views {
		data[i] = accountSummaryOf(v)
	}
	return gen.ListAccounts200JSONResponse{Data: data, Next: optional(next)}, nil
}

// GetAccount returns an account.
func (s *server) GetAccount(ctx context.Context, req gen.GetAccountRequestObject) (gen.GetAccountResponseObject, error) {
	v, err := s.accounts.Get(ctx, req.Ledger, req.Address)
	if err != nil {
		return nil, err
	}
	return gen.GetAccount200JSONResponse(accountOf(v)), nil
}

// ReplaceAccountMetadata replaces the metadata of an account.
func (s *server) ReplaceAccountMetadata(ctx context.Context, req gen.ReplaceAccountMetadataRequestObject) (gen.ReplaceAccountMetadataResponseObject, error) {
	v, err := s.accounts.ReplaceMetadata(ctx, req.Ledger, req.Address, metadataIn(req.Body))
	if err != nil {
		return nil, err
	}
	return gen.ReplaceAccountMetadata200JSONResponse(accountOf(v)), nil
}

// GetAccountHistory returns point-in-time balances.
func (s *server) GetAccountHistory(ctx context.Context, req gen.GetAccountHistoryRequestObject) (gen.GetAccountHistoryResponseObject, error) {
	points, err := s.accounts.History(ctx, req.Ledger, req.Address, req.Params.Asset, deref(req.Params.Limit))
	if err != nil {
		return nil, err
	}
	data := make([]gen.BalancePoint, len(points))
	for i, p := range points {
		data[i] = gen.BalancePoint{
			Timestamp: timestampOf(p.Timestamp),
			At:        tbTime(p.Timestamp),
			Balance:   signed(p.Balance),
			Reserved:  p.Reserved.String(),
			Credits:   p.Credits.String(),
			Debits:    p.Debits.String(),
		}
	}
	return gen.GetAccountHistory200JSONResponse{Address: req.Address, Asset: req.Params.Asset, Data: data}, nil
}

// AggregateBalances sums the balances under a prefix.
func (s *server) AggregateBalances(ctx context.Context, req gen.AggregateBalancesRequestObject) (gen.AggregateBalancesResponseObject, error) {
	agg, err := s.accounts.AggregateBalances(ctx, req.Ledger, req.Params.Address)
	if err != nil {
		return nil, err
	}
	assets := make([]gen.AggregateBalance, len(agg.Assets))
	for i, a := range agg.Assets {
		assets[i] = gen.AggregateBalance{
			Asset:     a.Asset,
			Accounts:  a.Accounts,
			Balance:   signed(a.Balance),
			Available: signed(a.Available),
			Reserved:  signed(a.Reserved),
			Incoming:  signed(a.Incoming),
		}
	}
	return gen.AggregateBalances200JSONResponse{Prefix: agg.Prefix, AsOf: agg.AsOf, Accounts: agg.Addresses, Chunks: agg.Chunks, Assets: assets}, nil
}

// ListSegments returns the child segments of a prefix.
func (s *server) ListSegments(ctx context.Context, req gen.ListSegmentsRequestObject) (gen.ListSegmentsResponseObject, error) {
	prefix, rows, err := s.accounts.Segments(ctx, req.Ledger, deref(req.Params.Prefix))
	if err != nil {
		return nil, err
	}
	data := make([]gen.Segment, len(rows))
	for i, r := range rows {
		data[i] = gen.Segment{Segment: r.Segment, Prefix: r.Prefix, Accounts: r.Accounts, Assets: assetsOf(r.Assets), IsAccount: r.IsAccount, HasChildren: r.HasChildren}
	}
	return gen.ListSegments200JSONResponse{Prefix: optional(prefix), Data: data}, nil
}
