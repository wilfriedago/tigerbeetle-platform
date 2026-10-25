package bootstrap

import (
	"log/slog"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/tigerbeetle"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
)

// modules holds the use cases, built in dependency order.
type modules struct {
	ledgers      *app.Ledgers
	settings     *app.Settings
	accounts     *app.Accounts
	transactions *app.Transactions
	holds        *app.Holds
	overview     *app.Overview
	reconciler   *app.Reconciler
}

func buildModules(r resources, cfg config.Config, logger *slog.Logger, o options) modules {
	metadata := postgres.New(r.db)
	ledgerStore := tigerbeetle.NewStore(r.tigerbeetle)
	clock := app.ClockFunc(time.Now)
	tb := o.wrapLedgerStore(ledgerStore)
	writes := app.TransactionsDeps{
		Store:       metadata,
		TigerBeetle: tb,
		IDs:         ledgerStore,
		Clock:       clock,
		GracePeriod: cfg.ReconcilerGracePeriod,
		Logger:      logger,
	}
	accounts := app.NewAccounts(metadata, metadata, metadata, tb, clock)
	transactions := app.NewTransactions(writes)
	holds := app.NewHolds(writes)
	return modules{
		ledgers:      app.NewLedgers(metadata, clock),
		settings:     app.NewSettings(metadata, metadata, metadata, metadata, metadata, clock),
		accounts:     accounts,
		transactions: transactions,
		holds:        holds,
		overview:     app.NewOverview(metadata, accounts, tb, clock),
		reconciler:   app.NewReconciler(metadata, transactions, holds, clock, cfg.ReconcilerGracePeriod, cfg.ReconcilerInterval, logger),
	}
}
