package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// registerOverview adds the bookkeeping operations under the tag ledgers: what a ledger holds.
func registerOverview(api huma.API, h *handlers) {
	tags := []string{"ledgers"}
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

func (h *handlers) getLedgerStats(ctx context.Context, in *restapi.LedgerPath) (*restapi.Body[dto.LedgerStats], error) {
	st, err := h.OverviewQueries.Stats(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.LedgerStats]{Body: dto.LedgerStatsFrom(&st)}, nil
}

func (h *handlers) listAssets(ctx context.Context, in *restapi.LedgerPath) (*restapi.Body[dto.AssetList], error) {
	assets, err := h.AccountQueries.Assets(ctx, string(in.Ledger))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.AssetList]{Body: dto.AssetList{Data: dto.AssetSummariesFrom(assets)}}, nil
}

func (h *handlers) search(ctx context.Context, in *struct {
	restapi.LedgerPath
	Q string `query:"q" required:"true" minLength:"1" maxLength:"255"`
},
) (*restapi.Body[dto.SearchResults], error) {
	r, err := h.OverviewQueries.Search(ctx, string(in.Ledger), in.Q)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.SearchResults]{Body: dto.SearchResultsFrom(in.Q, &r)}, nil
}
