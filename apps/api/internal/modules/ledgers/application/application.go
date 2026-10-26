package application

import (
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/domain"
)

// Deps are the ports the ledgers use cases need: the module's own repositories, and what the
// bookkeeping module answers.
type Deps struct {
	Ledgers    domain.LedgerRepository
	Rules      domain.RuleRepository
	EventTypes domain.EventTypeRepository
	Directory  *Directory
	Footprints Footprints
	Usage      EventTypeUsage
	Addresses  AddressAssets
	Clock      ledger.Clock
	Authorizer Authorizer
}

// Application is the ledgers module's use cases, as the transports see them.
type Application struct {
	LedgerCommands   LedgerCommands
	LedgerQueries    LedgerQueries
	SettingsCommands SettingsCommands
	SettingsQueries  SettingsQueries
}

// New builds the use cases, each behind the permission check of authorize.go: no transport can
// reach an unchecked use case.
func New(d Deps) Application {
	queries := &ledgerQueries{ledgers: d.Ledgers, footprints: d.Footprints, visible: d.Authorizer}
	return Application{
		LedgerCommands: authorizedLedgerCommands{auth: d.Authorizer, next: &ledgerCommands{ledgers: d.Ledgers, queries: queries, clock: d.Clock}},
		LedgerQueries:  authorizedLedgerQueries{auth: d.Authorizer, next: queries},
		SettingsCommands: authorizedSettingsCommands{auth: d.Authorizer, next: &settingsCommands{
			directory: d.Directory, rules: d.Rules, events: d.EventTypes, clock: d.Clock,
		}},
		SettingsQueries: authorizedSettingsQueries{auth: d.Authorizer, next: &settingsQueries{
			directory: d.Directory, rules: d.Rules, events: d.EventTypes, usage: d.Usage, addresses: d.Addresses,
		}},
	}
}
