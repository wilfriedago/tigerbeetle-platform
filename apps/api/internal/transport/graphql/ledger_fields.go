package graphql

import (
	"context"
	"time"

	bookkeepingdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	bookkeepinggql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/graphql"
	ledgersgql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/graphql"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// ledgerResolver resolves the fields of Ledger. The ledgers module owns the type and resolves its
// own fields (rules, ruleMatch, eventTypes); the bookkeeping module registers the fields about
// money on it, and receives only the ledger name from these one-line delegations, so it never
// imports the ledgers module (plan section 8.9).
type ledgerResolver struct {
	*ledgersgql.LedgerFields
	bookkeeping *bookkeepinggql.LedgerFields
}

func (r *ledgerResolver) Assets(ctx context.Context, obj *ledgersgql.Ledger) ([]bookkeepingdto.AssetSummary, error) {
	return r.bookkeeping.Assets(ctx, string(obj.Name))
}

func (r *ledgerResolver) Stats(ctx context.Context, obj *ledgersgql.Ledger) (*bookkeepingdto.LedgerStats, error) {
	return r.bookkeeping.Stats(ctx, string(obj.Name))
}

func (r *ledgerResolver) Accounts(ctx context.Context, obj *ledgersgql.Ledger, first *int, after, prefix, search, asset *string) (*bookkeepinggql.AccountConnection, error) {
	return r.bookkeeping.Accounts(ctx, string(obj.Name), first, after, prefix, search, asset)
}

func (r *ledgerResolver) Account(ctx context.Context, obj *ledgersgql.Ledger, address string) (*bookkeepingdto.Account, error) {
	return r.bookkeeping.Account(ctx, string(obj.Name), address)
}

func (r *ledgerResolver) AccountHistory(ctx context.Context, obj *ledgersgql.Ledger, address, asset string, limit *int) (*bookkeepingdto.BalanceHistory, error) {
	return r.bookkeeping.AccountHistory(ctx, string(obj.Name), address, asset, limit)
}

func (r *ledgerResolver) Segments(ctx context.Context, obj *ledgersgql.Ledger, prefix *string) ([]bookkeepingdto.Segment, error) {
	return r.bookkeeping.Segments(ctx, string(obj.Name), prefix)
}

func (r *ledgerResolver) AggregateBalances(ctx context.Context, obj *ledgersgql.Ledger, address string) (*bookkeepingdto.AggregateBalances, error) {
	return r.bookkeeping.AggregateBalances(ctx, string(obj.Name), address)
}

func (r *ledgerResolver) Transactions(ctx context.Context, obj *ledgersgql.Ledger, first *int, after, address, asset, reference, eventType *string,
	committedAfter, committedBefore *time.Time,
) (*bookkeepinggql.TransactionConnection, error) {
	return r.bookkeeping.Transactions(ctx, string(obj.Name), first, after, bookkeepinggql.TransactionFilters{
		Address: address, Asset: asset, Reference: reference, EventType: eventType,
		CommittedAfter: committedAfter, CommittedBefore: committedBefore,
	})
}

func (r *ledgerResolver) Transaction(ctx context.Context, obj *ledgersgql.Ledger, id scalar.ID) (*bookkeepingdto.Transaction, error) {
	return r.bookkeeping.Transaction(ctx, string(obj.Name), id)
}

func (r *ledgerResolver) Holds(ctx context.Context, obj *ledgersgql.Ledger, first *int, after *string, status *scalar.HoldStatus, address *string) (*bookkeepinggql.HoldConnection, error) {
	return r.bookkeeping.Holds(ctx, string(obj.Name), first, after, status, address)
}

func (r *ledgerResolver) HoldCounts(ctx context.Context, obj *ledgersgql.Ledger) (*bookkeepingdto.HoldStats, error) {
	return r.bookkeeping.HoldCounts(ctx, string(obj.Name))
}

func (r *ledgerResolver) Hold(ctx context.Context, obj *ledgersgql.Ledger, id scalar.ID) (*bookkeepingdto.Hold, error) {
	return r.bookkeeping.Hold(ctx, string(obj.Name), id)
}

func (r *ledgerResolver) Requests(ctx context.Context, obj *ledgersgql.Ledger, first *int, after *string, status *scalar.RequestStatus,
	operation *scalar.Operation, search, actor *string,
) (*bookkeepinggql.WriteRequestConnection, error) {
	return r.bookkeeping.Requests(ctx, string(obj.Name), first, after, status, operation, search, actor)
}

func (r *ledgerResolver) Request(ctx context.Context, obj *ledgersgql.Ledger, key string) (*bookkeepingdto.WriteRequestDetail, error) {
	return r.bookkeeping.Request(ctx, string(obj.Name), key)
}

func (r *ledgerResolver) Search(ctx context.Context, obj *ledgersgql.Ledger, q string) (*bookkeepingdto.SearchResults, error) {
	return r.bookkeeping.Search(ctx, string(obj.Name), q)
}
