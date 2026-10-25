// Package v1 is version 1 of the bookkeeping module's REST interface: accounts, transactions, write
// requests, holds, the ledger overview and the reconciler, written for Huma (ADR-0018). The
// operations keep the tags of the spec (getLedgerStats, listAssets and search are tagged ledgers,
// getReconcilerStatus operations), because tags drive the generated client. The handlers decode,
// call one use case and map the result with package dto; they hold no business rule.
package v1

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
)

// Deps holds the use cases the handlers call. cmd/openapi passes the zero value, because
// registering an operation never calls a use case.
type Deps struct {
	TransactionCommands application.TransactionCommands
	TransactionQueries  application.TransactionQueries
	HoldCommands        application.HoldCommands
	HoldQueries         application.HoldQueries
	AccountCommands     application.AccountCommands
	AccountQueries      application.AccountQueries
	RequestQueries      application.RequestQueries
	OverviewQueries     application.OverviewQueries
	Reconciler          application.ReconcilerQueries
}

type handlers struct {
	Deps
}

// Register adds the operations of the bookkeeping module to api.
func Register(api huma.API, deps Deps) {
	h := &handlers{Deps: deps}
	registerReconciler(api, h)
	registerOverview(api, h)
	registerAccounts(api, h)
	registerTransactions(api, h)
	registerHolds(api, h)
}
