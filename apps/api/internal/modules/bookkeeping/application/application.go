package application

import (
	"log/slog"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// Deps are the ports the bookkeeping use cases need: the module's repositories and ledger store,
// and what the ledgers module answers.
type Deps struct {
	Accounts     domain.AccountRepository
	Writes       domain.WriteRepository
	Transactions domain.TransactionRepository
	Holds        domain.HoldRepository
	Ledger       domain.LedgerStore
	IDs          domain.IDGenerator
	Clock        ledger.Clock
	Ledgers      LedgerDirectory
	Policies     AccountPolicies
	EventTypes   EventTypes
	// GracePeriod is how long a pending record is a write in progress (ADR-0007).
	GracePeriod time.Duration
	// ReconcilerInterval is how often the reconciler runs.
	ReconcilerInterval time.Duration
	Logger             *slog.Logger
	Authorizer         Authorizer
}

// Application is the bookkeeping module's use cases, as the transports and bootstrap see them.
type Application struct {
	TransactionCommands TransactionCommands
	TransactionQueries  TransactionQueries
	HoldCommands        HoldCommands
	HoldQueries         HoldQueries
	AccountCommands     AccountCommands
	AccountQueries      AccountQueries
	RequestQueries      RequestQueries
	OverviewQueries     OverviewQueries
	// ReconcilerQueries reports the reconciler of the organization of the call.
	ReconcilerQueries ReconcilerQueries
	// Footprints answers the ledgers module's ports.
	Footprints *FootprintQueries
	// Reconciler settles interrupted writes; bootstrap runs its loop.
	Reconciler *Reconciler
}

// New builds the use cases.
func New(d Deps) Application {
	grace := d.GracePeriod
	if grace <= 0 {
		grace = DefaultGracePeriod
	}
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	flow := &writeFlow{writes: d.Writes, tb: d.Ledger, clock: d.Clock, grace: grace, logger: logger}
	ensurer := &accountEnsurer{accounts: d.Accounts, policies: d.Policies, tb: d.Ledger, ids: d.IDs, clock: d.Clock}

	accounts := &accountQueries{ledgers: d.Ledgers, accounts: d.Accounts, holds: d.Holds, tb: d.Ledger, clock: d.Clock}
	transactions := &transactionQueries{ledgers: d.Ledgers, txs: d.Transactions, tb: d.Ledger}
	holds := &holdQueries{ledgers: d.Ledgers, holds: d.Holds, tb: d.Ledger, clock: d.Clock}
	txCommands := &transactionCommands{
		ledgers: d.Ledgers, events: d.EventTypes, txs: d.Transactions, accounts: ensurer, flow: flow,
		tb: d.Ledger, ids: d.IDs, clock: d.Clock, logger: logger, queries: transactions,
	}
	holdCmds := &holdCommands{
		ledgers: d.Ledgers, events: d.EventTypes, holds: d.Holds, accounts: ensurer, flow: flow,
		tb: d.Ledger, ids: d.IDs, clock: d.Clock, queries: holds,
	}
	reconciler := newReconciler(d.Writes, txCommands, holdCmds, d.Clock, grace, d.ReconcilerInterval, logger)
	// Every use case the transports see is behind its permission check (authorize.go). The
	// reconciler settles writes with the unwrapped commands: it acts for the platform itself.
	auth := d.Authorizer
	return Application{
		TransactionCommands: authorizedTransactionCommands{auth: auth, next: txCommands},
		TransactionQueries:  authorizedTransactionQueries{auth: auth, next: transactions},
		HoldCommands:        authorizedHoldCommands{auth: auth, next: holdCmds},
		HoldQueries:         authorizedHoldQueries{auth: auth, next: holds},
		AccountCommands: authorizedAccountCommands{auth: auth, next: &accountCommands{
			ledgers: d.Ledgers, accounts: d.Accounts, queries: accounts,
		}},
		AccountQueries: authorizedAccountQueries{auth: auth, next: accounts},
		RequestQueries: authorizedRequestQueries{auth: auth, next: &requestQueries{ledgers: d.Ledgers, writes: d.Writes}},
		OverviewQueries: authorizedOverviewQueries{auth: auth, next: &overviewQueries{
			ledgers: d.Ledgers, accounts: accounts, addrs: d.Accounts, writes: d.Writes, txs: d.Transactions,
			holds: d.Holds, tb: d.Ledger, clock: d.Clock,
		}},
		ReconcilerQueries: authorizedReconcilerQueries{auth: auth, next: reconciler},
		Footprints:        &FootprintQueries{accounts: d.Accounts, txs: d.Transactions},
		Reconciler:        reconciler,
	}
}
