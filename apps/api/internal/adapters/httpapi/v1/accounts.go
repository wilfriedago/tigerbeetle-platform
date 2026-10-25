package v1

import (
	"context"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// AssetBalance is one TigerBeetle account of an address, with its balances and frozen flags.
type AssetBalance struct {
	Asset     scalar.Asset        `json:"asset"`
	AccountID scalar.ID           `json:"accountId"`
	TbLedger  int64               `json:"tbLedger"`
	Balance   scalar.SignedAmount `json:"balance" doc:"Credits posted minus debits posted."`
	Available scalar.SignedAmount `json:"available" doc:"Balance minus reserved."`
	Reserved  scalar.Amount       `json:"reserved" doc:"Pending debits, the amount held by open holds."`
	Incoming  scalar.Amount       `json:"incoming" doc:"Pending credits. Not part of available."`
	Credits   scalar.Amount       `json:"credits" doc:"Total credits posted."`
	Debits    scalar.Amount       `json:"debits" doc:"Total debits posted."`
	Limit     scalar.BalanceLimit `json:"limit"`
	History   bool                `json:"history"`
	Code      int                 `json:"code" doc:"The TigerBeetle account code."`
	Rule      string              `json:"rule" doc:"The pattern of the rule that created the account."`
	CreatedAt time.Time           `json:"createdAt"`
}

// TransformSchema adds the description of AssetBalance.
func (AssetBalance) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Description = "One TigerBeetle account of an address, with its balances and frozen flags."
	return s
}

// AccountSummary is an address with its balance per asset.
type AccountSummary struct {
	Address   scalar.Address  `json:"address"`
	CreatedAt time.Time       `json:"createdAt"`
	Metadata  scalar.Metadata `json:"metadata"`
	Balances  []AssetBalance  `json:"balances"`
}

// AccountList is a page of accounts.
type AccountList struct {
	Data []AccountSummary `json:"data"`
	Next *string          `json:"next,omitempty"`
}

// Account is an account summary plus its open holds. Embedding keeps its JSON flat; the spec
// writes it as an allOf of AccountSummary and the extra member, as the committed spec does.
type Account struct {
	AccountSummary
	OpenHolds int64 `json:"openHolds" doc:"Open holds from this address."`
}

// accountExtra holds the members Account adds to AccountSummary.
type accountExtra struct {
	OpenHolds int64 `json:"openHolds" doc:"Open holds from this address."`
}

// Schema describes Account as AccountSummary plus openHolds.
func (Account) Schema(r huma.Registry) *huma.Schema {
	return component(r, "Account", func() *huma.Schema {
		return &huma.Schema{AllOf: []*huma.Schema{
			r.Schema(reflect.TypeFor[AccountSummary](), true, ""),
			inline[accountExtra](r),
		}}
	})
}

// BalancePoint is one point-in-time balance from TigerBeetle.
type BalancePoint struct {
	Timestamp scalar.Timestamp    `json:"timestamp"`
	At        time.Time           `json:"at"`
	Balance   scalar.SignedAmount `json:"balance"`
	Reserved  scalar.Amount       `json:"reserved"`
	Credits   scalar.Amount       `json:"credits"`
	Debits    scalar.Amount       `json:"debits"`
}

// BalanceHistory is the balance history of one account.
type BalanceHistory struct {
	Address scalar.Address `json:"address"`
	Asset   scalar.Asset   `json:"asset"`
	Data    []BalancePoint `json:"data"`
}

// AggregateBalance is the sum of one asset under a prefix.
type AggregateBalance struct {
	Asset     scalar.Asset        `json:"asset"`
	Accounts  int64               `json:"accounts"`
	Balance   scalar.SignedAmount `json:"balance"`
	Available scalar.SignedAmount `json:"available"`
	Reserved  scalar.Amount       `json:"reserved"`
	Incoming  scalar.Amount       `json:"incoming"`
}

// AggregateBalances are the per-asset sums under a prefix.
type AggregateBalances struct {
	Prefix   string             `json:"prefix"`
	AsOf     time.Time          `json:"asOf" doc:"When the last chunk was read."`
	Accounts int64              `json:"accounts" doc:"Addresses summed."`
	Chunks   int                `json:"chunks" doc:"TigerBeetle lookups made. More than one means the sum is not one snapshot."`
	Assets   []AggregateBalance `json:"assets"`
}

// Segment is the next address segment under a prefix.
type Segment struct {
	Segment     string         `json:"segment" example:"1234"`
	Prefix      string         `json:"prefix" doc:"The full path of the segment." example:"users:1234"`
	Accounts    int64          `json:"accounts" doc:"Addresses at or below the segment."`
	Assets      []scalar.Asset `json:"assets"`
	IsAccount   bool           `json:"isAccount" doc:"The prefix is itself an address."`
	HasChildren bool           `json:"hasChildren"`
}

// SegmentList lists the child segments of a prefix.
type SegmentList struct {
	Prefix *string   `json:"prefix,omitempty"`
	Data   []Segment `json:"data"`
}

func registerAccounts(api huma.API, h *handlers) {
	tags := []string{"accounts"}
	restapi.Register(api, huma.Operation{
		OperationID: "listAccounts", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/accounts", Tags: tags,
		Summary: "List accounts",
		Description: "Accounts in address order with their balance per asset, read from TigerBeetle in one batch\n" +
			"for the page. `prefix` matches whole segments (`users:1` does not match `users:12:wallet`);\n" +
			"`search` matches the start of the address as typed, for pickers. Lists cannot be sorted or\n" +
			"filtered by balance: Postgres holds no balances (ADR-0002).\n",
		Parameters: []*huma.Param{cursorParam()},
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
	LedgerPath
	Prefix scalar.AddressPrefix `query:"prefix" doc:"Only addresses equal to or below this prefix, on whole segments. A trailing colon is optional."`
	Search string               `query:"search" maxLength:"255" doc:"Only addresses starting with this text, for autocompletion."`
	Asset  scalar.Asset         `query:"asset" doc:"Only accounts holding this asset."`
	Page
},
) (*body[AccountList], error) {
	views, next, err := h.Accounts.List(ctx, string(in.Ledger), string(in.Prefix), in.Search, string(in.Asset), in.Cursor, in.Limit)
	if err != nil {
		return nil, err
	}
	out := AccountList{Data: make([]AccountSummary, len(views)), Next: optional(next)}
	for i := range views {
		out.Data[i] = accountSummaryOf(views[i])
	}
	return &body[AccountList]{Body: out}, nil
}

func (h *handlers) getAccount(ctx context.Context, in *struct {
	LedgerPath
	AccountPath
},
) (*body[Account], error) {
	v, err := h.Accounts.Get(ctx, string(in.Ledger), string(in.Address))
	if err != nil {
		return nil, err
	}
	return &body[Account]{Body: accountOf(v)}, nil
}

func (h *handlers) replaceAccountMetadata(ctx context.Context, in *struct {
	LedgerPath
	AccountPath
	Body scalar.Metadata
},
) (*body[Account], error) {
	v, err := h.Accounts.ReplaceMetadata(ctx, string(in.Ledger), string(in.Address), metadataIn(in.Body))
	if err != nil {
		return nil, err
	}
	return &body[Account]{Body: accountOf(v)}, nil
}

func (h *handlers) getAccountHistory(ctx context.Context, in *struct {
	LedgerPath
	AccountPath
	Asset scalar.Asset `query:"asset" required:"true"`
	Limit int          `query:"limit" minimum:"1" maximum:"200" default:"50"`
},
) (*body[BalanceHistory], error) {
	points, err := h.Accounts.History(ctx, string(in.Ledger), string(in.Address), string(in.Asset), in.Limit)
	if err != nil {
		return nil, err
	}
	out := BalanceHistory{Address: in.Address, Asset: in.Asset, Data: make([]BalancePoint, len(points))}
	for i, p := range points {
		out.Data[i] = BalancePoint{
			Timestamp: timestampOf(p.Timestamp),
			At:        tbTime(p.Timestamp),
			Balance:   signed(p.Balance),
			Reserved:  scalar.Amount(p.Reserved.String()),
			Credits:   scalar.Amount(p.Credits.String()),
			Debits:    scalar.Amount(p.Debits.String()),
		}
	}
	return &body[BalanceHistory]{Body: out}, nil
}

func (h *handlers) aggregateBalances(ctx context.Context, in *struct {
	LedgerPath
	Address scalar.AddressPrefix `query:"address" required:"true" doc:"The prefix, on whole segments. A trailing colon is optional."`
},
) (*body[AggregateBalances], error) {
	agg, err := h.Accounts.AggregateBalances(ctx, string(in.Ledger), string(in.Address))
	if err != nil {
		return nil, err
	}
	out := AggregateBalances{Prefix: agg.Prefix, AsOf: agg.AsOf, Accounts: agg.Addresses, Chunks: agg.Chunks, Assets: make([]AggregateBalance, len(agg.Assets))}
	for i, a := range agg.Assets {
		out.Assets[i] = AggregateBalance{
			Asset:     scalar.Asset(a.Asset),
			Accounts:  a.Accounts,
			Balance:   signed(a.Balance),
			Available: signed(a.Available),
			Reserved:  scalar.Amount(signed(a.Reserved)),
			Incoming:  scalar.Amount(signed(a.Incoming)),
		}
	}
	return &body[AggregateBalances]{Body: out}, nil
}

func (h *handlers) listSegments(ctx context.Context, in *struct {
	LedgerPath
	Prefix scalar.AddressPrefix `query:"prefix"`
},
) (*body[SegmentList], error) {
	prefix, rows, err := h.Accounts.Segments(ctx, string(in.Ledger), string(in.Prefix))
	if err != nil {
		return nil, err
	}
	out := SegmentList{Prefix: optional(prefix), Data: make([]Segment, len(rows))}
	for i, r := range rows {
		out.Data[i] = Segment{
			Segment: r.Segment, Prefix: r.Prefix, Accounts: r.Accounts, Assets: assetsOf(r.Assets),
			IsAccount: r.IsAccount, HasChildren: r.HasChildren,
		}
	}
	return &body[SegmentList]{Body: out}, nil
}

func balanceOf(b *app.BalanceView) AssetBalance {
	return AssetBalance{
		Asset:     scalar.Asset(b.Asset),
		AccountID: scalar.ID(b.ID.String()),
		TbLedger:  int64(b.TBLedger),
		Balance:   signed(b.Balance),
		Available: signed(b.Available),
		Reserved:  scalar.Amount(b.Reserved.String()),
		Incoming:  scalar.Amount(b.Incoming.String()),
		Credits:   scalar.Amount(b.Credits.String()),
		Debits:    scalar.Amount(b.Debits.String()),
		Limit:     scalar.BalanceLimit(b.Limit),
		History:   b.History,
		Code:      int(b.Code),
		Rule:      b.RulePattern,
		CreatedAt: b.CreatedAt,
	}
}

func accountSummaryOf(v app.AccountView) AccountSummary {
	balances := make([]AssetBalance, len(v.Balances))
	for i := range v.Balances {
		balances[i] = balanceOf(&v.Balances[i])
	}
	return AccountSummary{Address: scalar.Address(v.Address), CreatedAt: v.CreatedAt, Metadata: metadataOf(v.Metadata), Balances: balances}
}

func accountOf(v app.AccountView) Account {
	return Account{AccountSummary: accountSummaryOf(v), OpenHolds: v.OpenHolds}
}

// timestampOf renders a TigerBeetle timestamp as a decimal string.
func timestampOf(ts uint64) scalar.Timestamp {
	return scalar.Timestamp(strconv.FormatUint(ts, 10))
}

// tbTime turns a TigerBeetle timestamp, nanoseconds since the Unix epoch, into a time in UTC.
func tbTime(ts uint64) time.Time {
	return time.Unix(0, int64(ts)).UTC()
}
