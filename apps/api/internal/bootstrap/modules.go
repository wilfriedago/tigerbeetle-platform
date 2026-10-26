package bootstrap

import (
	"log/slog"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	accessapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	accessdomain "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
	accesskc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/infrastructure/keycloak"
	accesspg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/infrastructure/postgres"
	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	bookkeepingdomain "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	bookkeepingpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres"
	bookkeepingtb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/tigerbeetle"
	ledgersapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	ledgerspg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/infrastructure/postgres"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/keycloak"
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
func buildModules(r resources, cfg config.Config, logger *slog.Logger, o options) (modules, error) {
	clock := ledger.ClockFunc(time.Now)

	// Access comes first: every other module checks its permissions through it (ADR-0028).
	accessStore := accesspg.NewStore(r.db)
	dir, err := directory(cfg, o)
	if err != nil {
		return modules{}, err
	}
	ledgerStore := ledgerspg.NewStore(r.db)
	accessDeps := accessapp.Deps{
		Organizations: accessStore, Grants: accessStore, Sessions: accessStore, Directory: dir,
		Ledgers: ledgersapp.NewLedgerCounts(ledgerStore), Clock: clock,
	}
	ac := accessapp.New(accessDeps)

	ledgerDirectory := ledgersapp.NewDirectory(ledgerStore)
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
		Ledgers:            ledgerDirectory,
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
		Directory:  ledgerDirectory,
		Footprints: bk.Footprints,
		Usage:      bk.Footprints,
		Addresses:  bk.Footprints,
		Clock:      clock,
		Authorizer: ac.Authorizer,
	})
	return modules{access: ac, accessDeps: accessDeps, ledgers: lg, bookkeeping: bk, bookkeepingAccounts: bookStore}, nil
}

// directory is Keycloak's admin API for the access module, nil without AUTH_ADMIN_CLIENT_ID. It
// reaches Keycloak where the API reads discovery (AUTH_DISCOVERY_URL), and the console's client
// owns the pages after the account setup email.
func directory(cfg config.Config, o options) (accessdomain.Directory, error) {
	if o.directory != nil {
		return o.directory, nil
	}
	if cfg.AuthAdminClientID == "" {
		return nil, nil
	}
	realm := cfg.AuthDiscoveryURL
	if realm == "" {
		realm = cfg.AuthIssuerURL
	}
	admin, err := keycloak.New(keycloak.Config{
		RealmURL:      strings.TrimSuffix(strings.TrimSuffix(realm, "/"), "/.well-known/openid-configuration"),
		ClientID:      cfg.AuthAdminClientID,
		ClientSecret:  cfg.AuthAdminClientSecret,
		SetupClientID: cfg.AuthClientID,
	})
	if err != nil {
		return nil, err
	}
	return accesskc.New(admin, cfg.AuthIssuerURL), nil
}
