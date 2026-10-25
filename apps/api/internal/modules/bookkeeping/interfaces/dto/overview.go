package dto

import (
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// AssetAmount is an amount of one asset.
type AssetAmount struct {
	Asset  scalar.Asset  `json:"asset"`
	Amount scalar.Amount `json:"amount"`
}

// HoldStats counts the holds of a ledger by status.
type HoldStats struct {
	Open     int64         `json:"open"`
	Captured int64         `json:"captured"`
	Voided   int64         `json:"voided"`
	Expired  int64         `json:"expired"`
	Reserved []AssetAmount `json:"reserved" doc:"The amount held by open holds, per asset."`
}

// DeclinedReason counts the declined writes of one reason.
type DeclinedReason struct {
	Reason string `json:"reason" example:"exceeds-credits"`
	Count  int64  `json:"count"`
}

// declinedReason is DeclinedReason without its Schema method, for Inline.
type declinedReason DeclinedReason

// Schema keeps DeclinedReason inline in the spec, as an anonymous object.
func (DeclinedReason) Schema(r huma.Registry) *huma.Schema { return restapi.Inline[declinedReason](r) }

// DeclinedStats counts the declined and failed writes of the last 24 hours.
type DeclinedStats struct {
	Total   int64            `json:"total"`
	Reasons []DeclinedReason `json:"reasons"`
}

// TransformSchema adds the description of DeclinedStats.
func (DeclinedStats) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Description = "Declined and failed writes in the last 24 hours."
	return s
}

// AssetSummary is one asset of a ledger with its TigerBeetle ledger number.
type AssetSummary struct {
	Asset     scalar.Asset        `json:"asset"`
	TbLedger  int64               `json:"tbLedger" doc:"The TigerBeetle ledger number of this (ledger, asset) pair."`
	CreatedAt time.Time           `json:"createdAt" doc:"First use of the asset in the ledger."`
	Accounts  int64               `json:"accounts"`
	Issued    scalar.SignedAmount `json:"issued" doc:"Money issued, minus the balance of world, read from TigerBeetle."`
}

// LedgerStats are the figures of the ledger overview.
type LedgerStats struct {
	Accounts      int64          `json:"accounts"`
	AccountsToday int64          `json:"accountsToday" doc:"Accounts created since midnight UTC."`
	Assets        []AssetSummary `json:"assets"`
	Holds         HoldStats      `json:"holds"`
	Declined      DeclinedStats  `json:"declined"`
}

// SearchAccount is an address a search found.
type SearchAccount struct {
	Address scalar.Address `json:"address"`
	Assets  []scalar.Asset `json:"assets"`
}

type searchAccount SearchAccount

// Schema keeps SearchAccount inline in the spec.
func (SearchAccount) Schema(r huma.Registry) *huma.Schema { return restapi.Inline[searchAccount](r) }

// SearchTransaction is a transaction a search found.
type SearchTransaction struct {
	ID          scalar.ID `json:"id"`
	Reference   *string   `json:"reference,omitempty"`
	CommittedAt time.Time `json:"committedAt"`
}

type searchTransaction SearchTransaction

// Schema keeps SearchTransaction inline in the spec.
func (SearchTransaction) Schema(r huma.Registry) *huma.Schema {
	return restapi.Inline[searchTransaction](r)
}

// SearchHold is a hold a search found.
type SearchHold struct {
	ID     scalar.ID         `json:"id"`
	Status scalar.HoldStatus `json:"status"`
}

type searchHold SearchHold

// Schema keeps SearchHold inline in the spec.
func (SearchHold) Schema(r huma.Registry) *huma.Schema { return restapi.Inline[searchHold](r) }

// SearchRequest is a write request a search found.
type SearchRequest struct {
	Key       string               `json:"key"`
	Operation scalar.Operation     `json:"operation"`
	Status    scalar.RequestStatus `json:"status"`
}

type searchRequest SearchRequest

// Schema keeps SearchRequest inline in the spec.
func (SearchRequest) Schema(r huma.Registry) *huma.Schema { return restapi.Inline[searchRequest](r) }

// SearchResults are the matches of a search, by kind.
type SearchResults struct {
	Query        string              `json:"query"`
	Accounts     []SearchAccount     `json:"accounts"`
	Transactions []SearchTransaction `json:"transactions"`
	Holds        []SearchHold        `json:"holds"`
	Requests     []SearchRequest     `json:"requests"`
}

// AssetSummariesFrom renders the assets of a ledger.
func AssetSummariesFrom(assets []application.AssetView) []AssetSummary {
	out := make([]AssetSummary, len(assets))
	for i := range assets {
		a := &assets[i]
		out[i] = AssetSummary{
			Asset: scalar.Asset(a.Asset), TbLedger: int64(a.TBLedger), CreatedAt: a.CreatedAt,
			Accounts: a.Accounts, Issued: scalar.Signed(a.Issued),
		}
	}
	return out
}

// HoldStatsFrom counts holds by status. The reserved amounts are sorted by asset, so the answer
// does not depend on map order.
func HoldStatsFrom(c domain.HoldCounts, reserved map[string]*big.Int) HoldStats {
	s := HoldStats{Open: c.Open, Captured: c.Captured, Voided: c.Voided, Expired: c.Expired, Reserved: make([]AssetAmount, 0, len(reserved))}
	for asset, amount := range reserved {
		s.Reserved = append(s.Reserved, AssetAmount{Asset: scalar.Asset(asset), Amount: scalar.Amount(amount.String())})
	}
	slices.SortFunc(s.Reserved, func(a, b AssetAmount) int { return strings.Compare(string(a.Asset), string(b.Asset)) })
	return s
}

// LedgerStatsFrom renders the overview figures.
func LedgerStatsFrom(st *application.Stats) LedgerStats {
	out := LedgerStats{
		Accounts:      st.Accounts,
		AccountsToday: st.AccountsToday,
		Assets:        AssetSummariesFrom(st.Assets),
		Holds:         HoldStatsFrom(st.Holds, st.Reserved),
		Declined:      DeclinedStats{Total: st.DeclinedTotal, Reasons: make([]DeclinedReason, len(st.Declined))},
	}
	for i, d := range st.Declined {
		out.Declined.Reasons[i] = DeclinedReason{Reason: d.Reason, Count: d.Count}
	}
	return out
}

// SearchResultsFrom renders the matches of a search.
func SearchResultsFrom(query string, r *application.SearchResult) SearchResults {
	out := SearchResults{
		Query:        query,
		Accounts:     make([]SearchAccount, len(r.Accounts)),
		Transactions: make([]SearchTransaction, len(r.Transactions)),
		Holds:        make([]SearchHold, len(r.Holds)),
		Requests:     make([]SearchRequest, len(r.Requests)),
	}
	for i := range r.Accounts {
		a := &r.Accounts[i]
		assets := make([]scalar.Asset, len(a.Accounts))
		for k := range a.Accounts {
			assets[k] = scalar.Asset(a.Accounts[k].Asset)
		}
		out.Accounts[i] = SearchAccount{Address: scalar.Address(a.Address), Assets: assets}
	}
	for i := range r.Transactions {
		t := &r.Transactions[i]
		out.Transactions[i] = SearchTransaction{ID: scalar.IDOf(t.ID), Reference: restapi.Optional(t.Reference), CommittedAt: t.CommittedAt}
	}
	for i, h := range r.Holds {
		out.Holds[i] = SearchHold{ID: scalar.IDOf(h.ID), Status: scalar.HoldStatus(h.Status)}
	}
	for i := range r.Requests {
		w := &r.Requests[i]
		out.Requests[i] = SearchRequest{Key: w.Key, Operation: scalar.Operation(w.Operation), Status: scalar.RequestStatus(w.Status)}
	}
	return out
}
