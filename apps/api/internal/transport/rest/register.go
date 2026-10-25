// Package rest composes the REST operations of every module on one Huma API. It is the one list of
// REST operations: bootstrap serves it, and cmd/openapi writes packages/openapi/openapi.yaml from
// it. Transports compose modules; modules never import a transport.
package rest

import (
	"github.com/danielgtaylor/huma/v2"

	bookkeepingv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/rest/v1"
	healthv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/rest/v1"
	ledgersv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/rest/v1"
)

// Handlers holds what each module's operations call. cmd/openapi passes the zero value, because
// registering an operation never calls a handler.
type Handlers struct {
	Health      healthv1.Deps
	Ledgers     ledgersv1.Deps
	Bookkeeping bookkeepingv1.Deps
}

// RegisterAll adds every REST operation.
func RegisterAll(api huma.API, h Handlers) {
	RegisterHealth(api, h.Health)
	RegisterModules(api, h)
}

// RegisterHealth adds the health operations, which are served whatever REST_ENABLED says: the
// container healthcheck and load balancers depend on them.
func RegisterHealth(api huma.API, deps healthv1.Deps) {
	healthv1.Register(api, deps)
}

// RegisterModules adds the operations of the ledgers and bookkeeping modules.
func RegisterModules(api huma.API, h Handlers) {
	ledgersv1.Register(api, h.Ledgers)
	bookkeepingv1.Register(api, h.Bookkeeping)
}
