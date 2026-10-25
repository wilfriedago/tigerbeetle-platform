// Package v1 is version 1 of the ledgers module's REST interface: ledgers, account rules and event
// types, written for Huma (ADR-0018). The handlers decode, call one use case and map the result
// with package dto; they hold no business rule.
package v1

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
)

// Deps holds the use cases the handlers call. cmd/openapi passes the zero value, because
// registering an operation never calls a use case.
type Deps struct {
	LedgerCommands   application.LedgerCommands
	LedgerQueries    application.LedgerQueries
	SettingsCommands application.SettingsCommands
	SettingsQueries  application.SettingsQueries
}

type handlers struct {
	Deps
}

// Register adds the operations of the ledgers module to api.
func Register(api huma.API, deps Deps) {
	h := &handlers{Deps: deps}
	registerLedgers(api, h)
	registerSettings(api, h)
}
