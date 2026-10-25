// Package dto holds the wire types of the bookkeeping module that every transport shares: REST
// answers them, the broker RPCs reply with them, and GraphQL binds to them where its shapes match
// (plan section 8.8). The struct tags carry the descriptions and constraints of both specs.
package dto

import (
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
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

// Account is an account summary plus its open holds. Embedding keeps its JSON flat; the spec
// writes it as an allOf of AccountSummary and the extra member.
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
	return restapi.Component(r, "Account", func() *huma.Schema {
		return &huma.Schema{AllOf: []*huma.Schema{
			r.Schema(reflect.TypeFor[AccountSummary](), true, ""),
			restapi.Inline[accountExtra](r),
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

// BalanceFrom renders one account of an address.
func BalanceFrom(b *application.BalanceView) AssetBalance {
	return AssetBalance{
		Asset:     scalar.Asset(b.Asset),
		AccountID: scalar.IDOf(b.ID),
		TbLedger:  int64(b.TBLedger),
		Balance:   scalar.Signed(b.Balance),
		Available: scalar.Signed(b.Available),
		Reserved:  scalar.AmountOf(b.Reserved),
		Incoming:  scalar.AmountOf(b.Incoming),
		Credits:   scalar.AmountOf(b.Credits),
		Debits:    scalar.AmountOf(b.Debits),
		Limit:     scalar.BalanceLimit(b.Limit),
		History:   b.History,
		Code:      int(b.Code),
		Rule:      b.RulePattern,
		CreatedAt: b.CreatedAt,
	}
}

// AccountSummaryFrom renders an address with its balances.
func AccountSummaryFrom(v *application.AccountView) AccountSummary {
	balances := make([]AssetBalance, len(v.Balances))
	for i := range v.Balances {
		balances[i] = BalanceFrom(&v.Balances[i])
	}
	return AccountSummary{Address: scalar.Address(v.Address), CreatedAt: v.CreatedAt, Metadata: scalar.MetadataOf(v.Metadata), Balances: balances}
}

// AccountFrom renders an account with its open holds.
func AccountFrom(v *application.AccountView) Account {
	return Account{AccountSummary: AccountSummaryFrom(v), OpenHolds: v.OpenHolds}
}

// BalanceHistoryFrom renders the history of one account.
func BalanceHistoryFrom(address scalar.Address, asset scalar.Asset, points []application.HistoryPoint) BalanceHistory {
	out := BalanceHistory{Address: address, Asset: asset, Data: make([]BalancePoint, len(points))}
	for i := range points {
		p := &points[i]
		out.Data[i] = BalancePoint{
			Timestamp: scalar.TimestampOf(p.Timestamp),
			At:        domain.TimeFromTB(p.Timestamp),
			Balance:   scalar.Signed(p.Balance),
			Reserved:  scalar.AmountOf(p.Reserved),
			Credits:   scalar.AmountOf(p.Credits),
			Debits:    scalar.AmountOf(p.Debits),
		}
	}
	return out
}

// AggregateBalancesFrom renders a prefix sum.
func AggregateBalancesFrom(agg *application.Aggregate) AggregateBalances {
	out := AggregateBalances{Prefix: agg.Prefix, AsOf: agg.AsOf, Accounts: agg.Addresses, Chunks: agg.Chunks, Assets: make([]AggregateBalance, len(agg.Assets))}
	for i := range agg.Assets {
		a := &agg.Assets[i]
		out.Assets[i] = AggregateBalance{
			Asset:     scalar.Asset(a.Asset),
			Accounts:  a.Accounts,
			Balance:   scalar.Signed(a.Balance),
			Available: scalar.Signed(a.Available),
			Reserved:  scalar.Amount(scalar.Signed(a.Reserved)),
			Incoming:  scalar.Amount(scalar.Signed(a.Incoming)),
		}
	}
	return out
}

// SegmentFrom renders one child segment.
func SegmentFrom(r *domain.SegmentRow) Segment {
	return Segment{
		Segment: r.Segment, Prefix: r.Prefix, Accounts: r.Accounts, Assets: scalar.AssetsOf(r.Assets),
		IsAccount: r.IsAccount, HasChildren: r.HasChildren,
	}
}
