package httpapi

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
)

// ListLedgers returns a page of ledgers.
func (s *server) ListLedgers(ctx context.Context, req gen.ListLedgersRequestObject) (gen.ListLedgersResponseObject, error) {
	rows, next, err := s.ledgers.List(ctx, deref(req.Params.Cursor), deref(req.Params.Limit))
	if err != nil {
		return nil, err
	}
	data := make([]gen.Ledger, len(rows))
	for i, r := range rows {
		data[i] = ledgerOf(r)
	}
	return gen.ListLedgers200JSONResponse{Data: data, Next: optional(next)}, nil
}

// GetLedger returns a ledger.
func (s *server) GetLedger(ctx context.Context, req gen.GetLedgerRequestObject) (gen.GetLedgerResponseObject, error) {
	l, err := s.ledgers.Summary(ctx, req.Ledger)
	if err != nil {
		return nil, err
	}
	return gen.GetLedger200JSONResponse(ledgerOf(l)), nil
}

// CreateLedger creates a ledger.
func (s *server) CreateLedger(ctx context.Context, req gen.CreateLedgerRequestObject) (gen.CreateLedgerResponseObject, error) {
	var metadata map[string]string
	if req.Body != nil {
		metadata = metadataIn(req.Body.Metadata)
	}
	l, err := s.ledgers.Create(ctx, req.Ledger, metadata)
	if err != nil {
		return nil, err
	}
	return gen.CreateLedger201JSONResponse(ledgerOf(l)), nil
}

// ReplaceLedgerMetadata replaces the metadata of a ledger.
func (s *server) ReplaceLedgerMetadata(ctx context.Context, req gen.ReplaceLedgerMetadataRequestObject) (gen.ReplaceLedgerMetadataResponseObject, error) {
	l, err := s.ledgers.ReplaceMetadata(ctx, req.Ledger, metadataIn(req.Body))
	if err != nil {
		return nil, err
	}
	return gen.ReplaceLedgerMetadata200JSONResponse(ledgerOf(l)), nil
}

// GetLedgerStats returns the overview figures of a ledger.
func (s *server) GetLedgerStats(ctx context.Context, req gen.GetLedgerStatsRequestObject) (gen.GetLedgerStatsResponseObject, error) {
	st, err := s.overview.Stats(ctx, req.Ledger)
	if err != nil {
		return nil, err
	}
	body := gen.LedgerStats{
		Accounts:      st.Accounts,
		AccountsToday: st.AccountsToday,
		Assets:        assetSummaries(st.Assets),
		Holds:         holdStatsOf(st.Holds, st.Reserved),
		Declined:      gen.DeclinedStats{Total: st.DeclinedTotal},
	}
	body.Declined.Reasons = make([]struct {
		Count  int64  `json:"count"`
		Reason string `json:"reason"`
	}, len(st.Declined))
	for i, d := range st.Declined {
		body.Declined.Reasons[i].Reason = d.Reason
		body.Declined.Reasons[i].Count = d.Count
	}
	return gen.GetLedgerStats200JSONResponse(body), nil
}

func assetSummaries(assets []app.AssetView) []gen.AssetSummary {
	out := make([]gen.AssetSummary, len(assets))
	for i, a := range assets {
		out[i] = gen.AssetSummary{Asset: a.Asset, TbLedger: int64(a.TBLedger), CreatedAt: a.CreatedAt, Accounts: a.Accounts, Issued: signed(a.Issued)}
	}
	return out
}

// ListAssets returns the assets of a ledger.
func (s *server) ListAssets(ctx context.Context, req gen.ListAssetsRequestObject) (gen.ListAssetsResponseObject, error) {
	assets, err := s.accounts.Assets(ctx, req.Ledger)
	if err != nil {
		return nil, err
	}
	return gen.ListAssets200JSONResponse{Data: assetSummaries(assets)}, nil
}

// Search finds what a query names in a ledger.
func (s *server) Search(ctx context.Context, req gen.SearchRequestObject) (gen.SearchResponseObject, error) {
	r, err := s.overview.Search(ctx, req.Ledger, req.Params.Q)
	if err != nil {
		return nil, err
	}
	body := gen.SearchResults{Query: req.Params.Q}
	body.Accounts = make([]struct {
		Address gen.Address `json:"address"`
		Assets  []gen.Asset `json:"assets"`
	}, len(r.Accounts))
	for i, a := range r.Accounts {
		body.Accounts[i].Address = string(a.Address)
		body.Accounts[i].Assets = []gen.Asset{}
		for k := range a.Accounts {
			acc := &a.Accounts[k]
			body.Accounts[i].Assets = append(body.Accounts[i].Assets, acc.Asset)
		}
	}
	body.Transactions = make([]struct {
		CommittedAt time.Time `json:"committedAt"`
		Id          gen.Id    `json:"id"`
		Reference   *string   `json:"reference,omitempty"`
	}, len(r.Transactions))
	for i := range r.Transactions {
		t := &r.Transactions[i]
		body.Transactions[i].Id = t.ID.String()
		body.Transactions[i].CommittedAt = t.CommittedAt
		body.Transactions[i].Reference = optional(t.Reference)
	}
	body.Holds = make([]struct {
		Id     gen.Id         `json:"id"`
		Status gen.HoldStatus `json:"status"`
	}, len(r.Holds))
	for i := range r.Holds {
		h := &r.Holds[i]
		body.Holds[i].Id = h.ID.String()
		body.Holds[i].Status = gen.HoldStatus(s.overview.HoldStatusNow(*h))
	}
	body.Requests = make([]struct {
		Key       string            `json:"key"`
		Operation gen.Operation     `json:"operation"`
		Status    gen.RequestStatus `json:"status"`
	}, len(r.Requests))
	for i := range r.Requests {
		w := &r.Requests[i]
		body.Requests[i].Key = w.Key
		body.Requests[i].Operation = gen.Operation(w.Operation)
		body.Requests[i].Status = gen.RequestStatus(w.Status)
	}
	return gen.Search200JSONResponse(body), nil
}
