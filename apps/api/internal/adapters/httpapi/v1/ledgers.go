package v1

import (
	"context"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Ledger is a ledger with its metadata, assets and account count.
type Ledger struct {
	Name      scalar.LedgerName `json:"name"`
	Metadata  scalar.Metadata   `json:"metadata"`
	CreatedAt time.Time         `json:"createdAt"`
	Assets    []scalar.Asset    `json:"assets" doc:"The assets used so far, in canonical form."`
	Accounts  int64             `json:"accounts" doc:"The number of addresses in the ledger."`
}

// LedgerList is a page of ledgers.
type LedgerList struct {
	Data []Ledger `json:"data"`
	Next *string  `json:"next,omitempty" doc:"The cursor of the next page; absent on the last page."`
}

// CreateLedgerRequest is the body of POST /v1/ledgers/{ledger}.
type CreateLedgerRequest struct {
	Metadata scalar.Metadata `json:"metadata,omitempty"`
}

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

// declinedReason is DeclinedReason without its Schema method, for inline.
type declinedReason DeclinedReason

// Schema keeps DeclinedReason inline in the spec, as an anonymous object.
func (DeclinedReason) Schema(r huma.Registry) *huma.Schema { return inline[declinedReason](r) }

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

// AssetList lists the assets of a ledger.
type AssetList struct {
	Data []AssetSummary `json:"data"`
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
func (SearchAccount) Schema(r huma.Registry) *huma.Schema { return inline[searchAccount](r) }

// SearchTransaction is a transaction a search found.
type SearchTransaction struct {
	ID          scalar.ID `json:"id"`
	Reference   *string   `json:"reference,omitempty"`
	CommittedAt time.Time `json:"committedAt"`
}

type searchTransaction SearchTransaction

// Schema keeps SearchTransaction inline in the spec.
func (SearchTransaction) Schema(r huma.Registry) *huma.Schema { return inline[searchTransaction](r) }

// SearchHold is a hold a search found.
type SearchHold struct {
	ID     scalar.ID         `json:"id"`
	Status scalar.HoldStatus `json:"status"`
}

type searchHold SearchHold

// Schema keeps SearchHold inline in the spec.
func (SearchHold) Schema(r huma.Registry) *huma.Schema { return inline[searchHold](r) }

// SearchRequest is a write request a search found.
type SearchRequest struct {
	Key       string               `json:"key"`
	Operation scalar.Operation     `json:"operation"`
	Status    scalar.RequestStatus `json:"status"`
}

type searchRequest SearchRequest

// Schema keeps SearchRequest inline in the spec.
func (SearchRequest) Schema(r huma.Registry) *huma.Schema { return inline[searchRequest](r) }

// SearchResults are the matches of a search, by kind.
type SearchResults struct {
	Query        string              `json:"query"`
	Accounts     []SearchAccount     `json:"accounts"`
	Transactions []SearchTransaction `json:"transactions"`
	Holds        []SearchHold        `json:"holds"`
	Requests     []SearchRequest     `json:"requests"`
}

func registerLedgers(api huma.API, h *handlers) {
	tags := []string{"ledgers"}
	restapi.Register(api, huma.Operation{
		OperationID: "listLedgers", Method: http.MethodGet, Path: "/v1/ledgers", Tags: tags,
		Summary:     "List ledgers",
		Description: "Every ledger in name order, with its assets and account count.",
		Parameters:  []*huma.Param{cursorParam()},
		Responses:   restapi.Success(http.StatusOK, "A page of ledgers."),
	}, h.listLedgers)

	restapi.Register(api, huma.Operation{
		OperationID: "getLedger", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}", Tags: tags,
		Summary:     "Get a ledger",
		Description: "The ledger with its metadata, assets and account count.",
		Responses:   restapi.Success(http.StatusOK, "The ledger."),
	}, h.getLedger)

	restapi.Register(api, huma.Operation{
		OperationID: "createLedger", Method: http.MethodPost, Path: "/v1/ledgers/{ledger}", Tags: tags,
		Summary: "Create a ledger",
		Description: "Creates a ledger. The name is permanent and appears in every API path. A ledger cannot be\n" +
			"deleted, because TigerBeetle accounts and transfers are permanent. Answers 409 when the\n" +
			"name is taken.\n",
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The ledger was created."),
	}, h.createLedger)

	restapi.Register(api, huma.Operation{
		OperationID: "replaceLedgerMetadata", Method: http.MethodPut, Path: "/v1/ledgers/{ledger}/metadata", Tags: tags,
		Summary:     "Replace ledger metadata",
		Description: "Replaces the metadata of the ledger with the given key/value pairs.",
		Responses:   restapi.Success(http.StatusOK, "The ledger with its new metadata."),
	}, h.replaceLedgerMetadata)

	restapi.Register(api, huma.Operation{
		OperationID: "getLedgerStats", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/stats", Tags: tags,
		Summary: "Ledger overview figures",
		Description: "The figures of the ledger overview: money issued per asset (minus the balance of `world`,\n" +
			"read from TigerBeetle), account and hold counts, reserved amounts, and the declined writes\n" +
			"of the last 24 hours with their reasons.\n",
		Responses: restapi.Success(http.StatusOK, "Overview figures."),
	}, h.getLedgerStats)

	restapi.Register(api, huma.Operation{
		OperationID: "listAssets", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/assets", Tags: tags,
		Summary: "List assets",
		Description: "The assets used in the ledger, each with its TigerBeetle ledger number (ADR-0005), its\n" +
			"account count and the money issued. A scale never changes: a new scale is a new asset.\n",
		Responses: restapi.Success(http.StatusOK, "The assets of the ledger."),
	}, h.listAssets)

	restapi.Register(api, huma.Operation{
		OperationID: "search", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/search", Tags: tags,
		Summary: "Search the ledger",
		Description: "Finds what a query names: a 32-hex ID opens a transaction or hold, an address or prefix\n" +
			"finds accounts, a reference finds transactions, an Idempotency-Key finds a write request.\n",
		Responses: restapi.Success(http.StatusOK, "Matches by kind."),
	}, h.search)
}

func (h *handlers) listLedgers(ctx context.Context, in *struct{ Page }) (*body[LedgerList], error) {
	rows, next, err := h.Ledgers.List(ctx, in.Cursor, in.Limit)
	if err != nil {
		return nil, err
	}
	out := LedgerList{Data: make([]Ledger, len(rows)), Next: optional(next)}
	for i, r := range rows {
		out.Data[i] = ledgerOf(r)
	}
	return &body[LedgerList]{Body: out}, nil
}

func (h *handlers) getLedger(ctx context.Context, in *LedgerPath) (*body[Ledger], error) {
	l, err := h.Ledgers.Summary(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	return &body[Ledger]{Body: ledgerOf(l)}, nil
}

func (h *handlers) createLedger(ctx context.Context, in *struct {
	LedgerPath
	Body CreateLedgerRequest
},
) (*body[Ledger], error) {
	l, err := h.Ledgers.Create(ctx, string(in.Ledger), metadataIn(in.Body.Metadata))
	if err != nil {
		return nil, err
	}
	return &body[Ledger]{Body: ledgerOf(l)}, nil
}

func (h *handlers) replaceLedgerMetadata(ctx context.Context, in *struct {
	LedgerPath
	Body scalar.Metadata
},
) (*body[Ledger], error) {
	l, err := h.Ledgers.ReplaceMetadata(ctx, string(in.Ledger), metadataIn(in.Body))
	if err != nil {
		return nil, err
	}
	return &body[Ledger]{Body: ledgerOf(l)}, nil
}

func (h *handlers) getLedgerStats(ctx context.Context, in *LedgerPath) (*body[LedgerStats], error) {
	st, err := h.Overview.Stats(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	out := LedgerStats{
		Accounts:      st.Accounts,
		AccountsToday: st.AccountsToday,
		Assets:        assetSummaries(st.Assets),
		Holds:         holdStatsOf(st.Holds, st.Reserved),
		Declined:      DeclinedStats{Total: st.DeclinedTotal, Reasons: make([]DeclinedReason, len(st.Declined))},
	}
	for i, d := range st.Declined {
		out.Declined.Reasons[i] = DeclinedReason{Reason: d.Reason, Count: d.Count}
	}
	return &body[LedgerStats]{Body: out}, nil
}

func (h *handlers) listAssets(ctx context.Context, in *LedgerPath) (*body[AssetList], error) {
	assets, err := h.Accounts.Assets(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	return &body[AssetList]{Body: AssetList{Data: assetSummaries(assets)}}, nil
}

func (h *handlers) search(ctx context.Context, in *struct {
	LedgerPath
	Q string `query:"q" required:"true" minLength:"1" maxLength:"255"`
},
) (*body[SearchResults], error) {
	r, err := h.Overview.Search(ctx, string(in.Ledger), in.Q)
	if err != nil {
		return nil, err
	}
	out := SearchResults{
		Query:        in.Q,
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
		out.Transactions[i] = SearchTransaction{ID: scalar.ID(t.ID.String()), Reference: optional(t.Reference), CommittedAt: t.CommittedAt}
	}
	for i := range r.Holds {
		hold := &r.Holds[i]
		out.Holds[i] = SearchHold{ID: scalar.ID(hold.ID.String()), Status: scalar.HoldStatus(h.Overview.HoldStatusNow(*hold))}
	}
	for i := range r.Requests {
		w := &r.Requests[i]
		out.Requests[i] = SearchRequest{Key: w.Key, Operation: scalar.Operation(w.Operation), Status: scalar.RequestStatus(w.Status)}
	}
	return &body[SearchResults]{Body: out}, nil
}

func ledgerOf(s app.LedgerSummary) Ledger {
	return Ledger{
		Name:      scalar.LedgerName(s.Name),
		Metadata:  metadataOf(s.Metadata),
		CreatedAt: s.CreatedAt,
		Assets:    assetsOf(s.Assets),
		Accounts:  s.Accounts,
	}
}

func assetSummaries(assets []app.AssetView) []AssetSummary {
	out := make([]AssetSummary, len(assets))
	for i, a := range assets {
		out[i] = AssetSummary{
			Asset: scalar.Asset(a.Asset), TbLedger: int64(a.TBLedger), CreatedAt: a.CreatedAt,
			Accounts: a.Accounts, Issued: signed(a.Issued),
		}
	}
	return out
}

// holdStatsOf counts holds by status. The reserved amounts are sorted by asset, so the answer
// does not depend on map order.
func holdStatsOf(c app.HoldCounts, reserved map[string]*big.Int) HoldStats {
	s := HoldStats{Open: c.Open, Captured: c.Captured, Voided: c.Voided, Expired: c.Expired, Reserved: make([]AssetAmount, 0, len(reserved))}
	for asset, amount := range reserved {
		s.Reserved = append(s.Reserved, AssetAmount{Asset: scalar.Asset(asset), Amount: scalar.Amount(amount.String())})
	}
	slices.SortFunc(s.Reserved, func(a, b AssetAmount) int { return strings.Compare(string(a.Asset), string(b.Asset)) })
	return s
}

// signed renders a balance exactly, never through a float (ADR-0004).
func signed(v *big.Int) scalar.SignedAmount {
	if v == nil {
		return "0"
	}
	return scalar.SignedAmount(v.String())
}

// metadataOf answers {} rather than null for metadata never set.
func metadataOf(m map[string]string) scalar.Metadata {
	if m == nil {
		return scalar.Metadata{}
	}
	return m
}

// metadataIn passes {} rather than nil to the use cases for metadata absent from a request.
func metadataIn(m scalar.Metadata) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// assetsOf answers [] rather than null for a ledger without assets.
func assetsOf(assets []string) []scalar.Asset {
	out := make([]scalar.Asset, len(assets))
	for i, a := range assets {
		out[i] = scalar.Asset(a)
	}
	return out
}
