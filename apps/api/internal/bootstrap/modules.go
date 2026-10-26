package bootstrap

import (
	"log/slog"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	accessapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	accesspg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/infrastructure/postgres"
	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	bookkeepingdomain "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	bookkeepingpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres"
	bookkeepingtb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/tigerbeetle"
	ledgersapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	ledgerspg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/infrastructure/postgres"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
)

// modules holds the use cases of every module.
type modules struct {
	access      accessapp.Application
	accessDeps  accessapp.Deps
	ledgers     ledgersapp.Application
	bookkeeping bookkeepingapp.Application
	// bookkeepingAccounts is what the ledger event relay enriches events from (ADR-0022).
	bookkeepingAccounts bookkeepingdomain.AccountRepository
}

// buildModules builds the use cases in dependency order (plan section 4.5). The modules depend on
// each other in both directions, but no use case does: the ledgers queries bookkeeping needs read
// only ledgers tables, and the bookkeeping queries ledgers needs read only bookkeeping tables. So
// the directory and policies of ledgers come first, then bookkeeping, then the rest of ledgers,
// which uses bookkeeping's footprint queries. Each value passed to a port that another module
// declares is checked by the compiler on the line that passes it.
func buildModules(r resources, cfg config.Config, logger *slog.Logger, o options) modules {
	clock := ledger.ClockFunc(time.Now)

	// Access comes first: every other module checks its permissions through it (ADR-0027).
	accessStore := accesspg.NewStore(r.db)
	accessDeps := accessapp.Deps{Organizations: accessStore, Grants: accessStore, Sessions: accessStore, Clock: clock}
	ac := accessapp.New(accessDeps)

	ledgerStore := ledgerspg.NewStore(r.db)
	directory := ledgersapp.NewDirectory(ledgerStore)
	policies := ledgersapp.NewPolicyQueries(ledgerStore, ledgerStore)

	bookStore := bookkeepingpg.NewStore(r.db)
	tbStore := bookkeepingtb.NewStore(r.tigerbeetle)
	bk := bookkeepingapp.New(bookkeepingapp.Deps{
		Accounts:           bookStore,
		Writes:             bookStore,
		Transactions:       bookStore,
		Holds:              bookStore,
		Ledger:             o.wrapLedgerStore(tbStore),
		IDs:                tbStore,
		Clock:              clock,
		Ledgers:            directory,
		Policies:           policies,
		EventTypes:         policies,
		GracePeriod:        cfg.ReconcilerGracePeriod,
		ReconcilerInterval: cfg.ReconcilerInterval,
		Logger:             logger,
		Authorizer:         ac.Authorizer,
	})

	lg := ledgersapp.New(ledgersapp.Deps{
		Ledgers:    ledgerStore,
		Rules:      ledgerStore,
		EventTypes: ledgerStore,
		Directory:  directory,
		Footprints: bk.Footprints,
		Usage:      bk.Footprints,
		Addresses:  bk.Footprints,
		Clock:      clock,
		Authorizer: ac.Authorizer,
	})
	return modules{access: ac, accessDeps: accessDeps, ledgers: lg, bookkeeping: bk, bookkeepingAccounts: bookStore}
}
