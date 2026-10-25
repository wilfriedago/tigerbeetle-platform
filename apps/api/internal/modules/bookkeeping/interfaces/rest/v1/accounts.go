package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

func registerAccounts(api huma.API, h *handlers) {
	tags := []string{"accounts"}
	restapi.Register(api, huma.Operation{
		OperationID: "listAccounts", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/accounts", Tags: tags,
		Summary: "List accounts",
		Description: "Accounts in address order with their balance per asset, read from TigerBeetle in one batch\n" +
			"for the page. `prefix` matches whole segments (`users:1` does not match `users:12:wallet`);\n" +
			"`search` matches the start of the address as typed, for pickers. Lists cannot be sorted or\n" +
			"filtered by balance: Postgres holds no balances (ADR-0002).\n",
		Parameters: []*huma.Param{restapi.CursorParam()},
		Responses:  restapi.Success(http.StatusOK, "A page of accounts."),
	}, h.listAccounts)

	restapi.Register(api, huma.Operation{
		OperationID: "getAccount", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/accounts/{address}", Tags: tags,
		Summary: "Get an account",
		Description: "The account with one entry per asset: the TigerBeetle account ID, balance, reserved,\n" +
			"available, incoming and volumes read from TigerBeetle, and the flags frozen at creation.\n",
		Parameters: []*huma.Param{addressParam(api)},
		Responses:  restapi.Success(http.StatusOK, "The account."),
	}, h.getAccount)

	restapi.Register(api, huma.Operation{
		OperationID: "replaceAccountMetadata", Method: http.MethodPut, Path: "/v1/ledgers/{ledger}/accounts/{address}/metadata", Tags: tags,
		Summary:     "Replace account metadata",
		Description: "Replaces the metadata of an existing account.",
		Parameters:  []*huma.Param{addressParam(api)},
		Responses:   restapi.Success(http.StatusOK, "The account with its new metadata."),
	}, h.replaceAccountMetadata)

	restapi.Register(api, huma.Operation{
		OperationID: "getAccountHistory", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/accounts/{address}/history", Tags: tags,
		Summary: "Balance history",
		Description: "Point-in-time balances from TigerBeetle `get_account_balances`, newest first. Only accounts\n" +
			"created with the `history` flag keep them; the flag cannot be added later, so it is a\n" +
			"property of the account rule (ADR-0006). Answers 422 `history-not-enabled` otherwise.\n",
		Parameters: []*huma.Param{addressParam(api)},
		Responses:  restapi.Success(http.StatusOK, "Balance history."),
	}, h.getAccountHistory)

	restapi.Register(api, huma.Operation{
		OperationID: "aggregateBalances", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/aggregate/balances", Tags: tags,
		Summary: "Sum balances under a prefix",
		Description: "Sums the balances of every account under an address prefix, per asset. TigerBeetle has no\n" +
			"prefix queries: Postgres resolves the prefix to account IDs and the service reads them in\n" +
			"chunks of up to 8189, so the total is not one consistent snapshot across chunks. A prefix\n" +
			"covering more accounts than the cap answers 422 `prefix-too-broad`.\n",
		Responses: restapi.Success(http.StatusOK, "Per-asset sums."),
	}, h.aggregateBalances)

	restapi.Register(api, huma.Operation{
		OperationID: "listSegments", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/aggregate/segments", Tags: tags,
		Summary: "Address segments under a prefix",
		Description: "The next address segment under a prefix, with the number of accounts below each one: the\n" +
			"chart of accounts by segment. Without a prefix, the top-level segments.\n",
		Responses: restapi.Success(http.StatusOK, "Child segments."),
	}, h.listSegments)
}

func (h *handlers) listAccounts(ctx context.Context, in *struct {
	restapi.LedgerPath
	Prefix scalar.AddressPrefix `query:"prefix" doc:"Only addresses equal to or below this prefix, on whole segments. A trailing colon is optional."`
	Search string               `query:"search" maxLength:"255" doc:"Only addresses starting with this text, for autocompletion."`
	Asset  scalar.Asset         `query:"asset" doc:"Only accounts holding this asset."`
	restapi.Page
},
) (*restapi.Body[dto.AccountList], error) {
	views, next, err := h.AccountQueries.List(ctx, application.ListAccountsInput{
		Ledger: string(in.Ledger), Prefix: string(in.Prefix), Search: in.Search, Asset: string(in.Asset), Cursor: in.Cursor, Limit: in.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := dto.AccountList{Data: make([]dto.AccountSummary, len(views)), Next: restapi.Optional(next)}
	for i := range views {
		out.Data[i] = dto.AccountSummaryFrom(&views[i])
	}
	return &restapi.Body[dto.AccountList]{Body: out}, nil
}

func (h *handlers) getAccount(ctx context.Context, in *struct {
	restapi.LedgerPath
	AccountPath
},
) (*restapi.Body[dto.Account], error) {
	v, err := h.AccountQueries.Get(ctx, string(in.Ledger), string(in.Address))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Account]{Body: dto.AccountFrom(&v)}, nil
}

func (h *handlers) replaceAccountMetadata(ctx context.Context, in *struct {
	restapi.LedgerPath
	AccountPath
	Body scalar.Metadata
},
) (*restapi.Body[dto.Account], error) {
	v, err := h.AccountCommands.ReplaceMetadata(ctx, string(in.Ledger), string(in.Address), in.Body.Map())
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Account]{Body: dto.AccountFrom(&v)}, nil
}

func (h *handlers) getAccountHistory(ctx context.Context, in *struct {
	restapi.LedgerPath
	AccountPath
	Asset scalar.Asset `query:"asset" required:"true"`
	Limit int          `query:"limit" minimum:"1" maximum:"200" default:"50"`
},
) (*restapi.Body[dto.BalanceHistory], error) {
	points, err := h.AccountQueries.History(ctx, string(in.Ledger), string(in.Address), string(in.Asset), in.Limit)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.BalanceHistory]{Body: dto.BalanceHistoryFrom(in.Address, in.Asset, points)}, nil
}

func (h *handlers) aggregateBalances(ctx context.Context, in *struct {
	restapi.LedgerPath
	Address scalar.AddressPrefix `query:"address" required:"true" doc:"The prefix, on whole segments. A trailing colon is optional."`
},
) (*restapi.Body[dto.AggregateBalances], error) {
	agg, err := h.AccountQueries.AggregateBalances(ctx, string(in.Ledger), string(in.Address))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.AggregateBalances]{Body: dto.AggregateBalancesFrom(&agg)}, nil
}

func (h *handlers) listSegments(ctx context.Context, in *struct {
	restapi.LedgerPath
	Prefix scalar.AddressPrefix `query:"prefix"`
},
) (*restapi.Body[dto.SegmentList], error) {
	prefix, rows, err := h.AccountQueries.Segments(ctx, string(in.Ledger), string(in.Prefix))
	if err != nil {
		return nil, err
	}
	out := dto.SegmentList{Prefix: restapi.Optional(prefix), Data: make([]dto.Segment, len(rows))}
	for i := range rows {
		out.Data[i] = dto.SegmentFrom(&rows[i])
	}
	return &restapi.Body[dto.SegmentList]{Body: out}, nil
}
