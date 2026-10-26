package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// The use cases behind their permission checks (ADR-0027). Each type wraps one use-case interface
// and checks the permission before it calls the real use case; New returns only these, so every
// transport gets the same answer. TestEveryUseCaseIsChecked fails when a method has no check.
//
// Go note: a type with the same methods as an interface, holding another value of that
// interface, is the decorator pattern. Go has no annotations, so this replaces @PreAuthorize.

type authorizedTransactionCommands struct {
	auth Authorizer
	next TransactionCommands
}

func (a authorizedTransactionCommands) Create(ctx context.Context, in CreateTransactionInput) (TransactionView, error) {
	if err := a.auth.Require(ctx, ledger.PermMoneyWrite, in.Ledger); err != nil {
		return TransactionView{}, err
	}
	return a.next.Create(ctx, in)
}

func (a authorizedTransactionCommands) Revert(ctx context.Context, in RevertTransactionInput) (TransactionView, error) {
	if err := a.auth.Require(ctx, ledger.PermMoneyWrite, in.Ledger); err != nil {
		return TransactionView{}, err
	}
	return a.next.Revert(ctx, in)
}

func (a authorizedTransactionCommands) ReplaceMetadata(ctx context.Context, ledgerName, id string, metadata map[string]string) (TransactionView, error) {
	if err := a.auth.Require(ctx, ledger.PermMetadataWrite, ledgerName); err != nil {
		return TransactionView{}, err
	}
	return a.next.ReplaceMetadata(ctx, ledgerName, id, metadata)
}

type authorizedTransactionQueries struct {
	auth Authorizer
	next TransactionQueries
}

func (a authorizedTransactionQueries) Get(ctx context.Context, ledgerName, id string) (TransactionView, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return TransactionView{}, err
	}
	return a.next.Get(ctx, ledgerName, id)
}

func (a authorizedTransactionQueries) List(ctx context.Context, in ListTransactionsInput) ([]TransactionView, string, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, in.Ledger); err != nil {
		return nil, "", err
	}
	return a.next.List(ctx, in)
}

type authorizedHoldCommands struct {
	auth Authorizer
	next HoldCommands
}

func (a authorizedHoldCommands) Create(ctx context.Context, in CreateHoldInput) (HoldView, error) {
	if err := a.auth.Require(ctx, ledger.PermMoneyWrite, in.Ledger); err != nil {
		return HoldView{}, err
	}
	return a.next.Create(ctx, in)
}

func (a authorizedHoldCommands) Capture(ctx context.Context, in CaptureHoldInput) (HoldView, error) {
	if err := a.auth.Require(ctx, ledger.PermMoneyWrite, in.Ledger); err != nil {
		return HoldView{}, err
	}
	return a.next.Capture(ctx, in)
}

func (a authorizedHoldCommands) Void(ctx context.Context, in VoidHoldInput) (HoldView, error) {
	if err := a.auth.Require(ctx, ledger.PermMoneyWrite, in.Ledger); err != nil {
		return HoldView{}, err
	}
	return a.next.Void(ctx, in)
}

type authorizedHoldQueries struct {
	auth Authorizer
	next HoldQueries
}

func (a authorizedHoldQueries) Get(ctx context.Context, ledgerName, id string) (HoldView, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return HoldView{}, err
	}
	return a.next.Get(ctx, ledgerName, id)
}

func (a authorizedHoldQueries) List(ctx context.Context, in ListHoldsInput) ([]HoldView, string, domain.HoldCounts, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, in.Ledger); err != nil {
		return nil, "", domain.HoldCounts{}, err
	}
	return a.next.List(ctx, in)
}

func (a authorizedHoldQueries) Counts(ctx context.Context, ledgerName string) (domain.HoldCounts, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return domain.HoldCounts{}, err
	}
	return a.next.Counts(ctx, ledgerName)
}

type authorizedAccountCommands struct {
	auth Authorizer
	next AccountCommands
}

func (a authorizedAccountCommands) ReplaceMetadata(ctx context.Context, ledgerName, address string, metadata map[string]string) (AccountView, error) {
	if err := a.auth.Require(ctx, ledger.PermMetadataWrite, ledgerName); err != nil {
		return AccountView{}, err
	}
	return a.next.ReplaceMetadata(ctx, ledgerName, address, metadata)
}

type authorizedAccountQueries struct {
	auth Authorizer
	next AccountQueries
}

func (a authorizedAccountQueries) List(ctx context.Context, in ListAccountsInput) ([]AccountView, string, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, in.Ledger); err != nil {
		return nil, "", err
	}
	return a.next.List(ctx, in)
}

func (a authorizedAccountQueries) Get(ctx context.Context, ledgerName, address string) (AccountView, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return AccountView{}, err
	}
	return a.next.Get(ctx, ledgerName, address)
}

func (a authorizedAccountQueries) History(ctx context.Context, ledgerName, address, asset string, limit int) ([]HistoryPoint, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return nil, err
	}
	return a.next.History(ctx, ledgerName, address, asset, limit)
}

func (a authorizedAccountQueries) AggregateBalances(ctx context.Context, ledgerName, prefix string) (Aggregate, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return Aggregate{}, err
	}
	return a.next.AggregateBalances(ctx, ledgerName, prefix)
}

func (a authorizedAccountQueries) Segments(ctx context.Context, ledgerName, prefix string) (string, []domain.SegmentRow, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return "", nil, err
	}
	return a.next.Segments(ctx, ledgerName, prefix)
}

func (a authorizedAccountQueries) Assets(ctx context.Context, ledgerName string) ([]AssetView, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return nil, err
	}
	return a.next.Assets(ctx, ledgerName)
}

type authorizedRequestQueries struct {
	auth Authorizer
	next RequestQueries
}

func (a authorizedRequestQueries) List(ctx context.Context, in ListRequestsInput) ([]domain.WriteRecord, string, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, in.Ledger); err != nil {
		return nil, "", err
	}
	return a.next.List(ctx, in)
}

func (a authorizedRequestQueries) Get(ctx context.Context, ledgerName, key string) (domain.WriteRecord, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return domain.WriteRecord{}, err
	}
	return a.next.Get(ctx, ledgerName, key)
}

type authorizedOverviewQueries struct {
	auth Authorizer
	next OverviewQueries
}

func (a authorizedOverviewQueries) Stats(ctx context.Context, ledgerName string) (Stats, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return Stats{}, err
	}
	return a.next.Stats(ctx, ledgerName)
}

func (a authorizedOverviewQueries) Search(ctx context.Context, ledgerName, query string) (SearchResult, error) {
	if err := a.auth.Require(ctx, ledger.PermLedgerRead, ledgerName); err != nil {
		return SearchResult{}, err
	}
	return a.next.Search(ctx, ledgerName, query)
}

type authorizedReconcilerQueries struct {
	auth Authorizer
	next ReconcilerQueries
}

func (a authorizedReconcilerQueries) Status(ctx context.Context) (ReconcilerStatus, error) {
	if err := a.auth.Require(ctx, ledger.PermOrgOperate, ""); err != nil {
		return ReconcilerStatus{}, err
	}
	return a.next.Status(ctx)
}
