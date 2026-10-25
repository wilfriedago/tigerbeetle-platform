// Package rpc registers every module's broker RPC methods on one router (ADR-0021), the broker
// counterpart of internal/transport/rest. The same router serves AMQP and NATS.
package rpc

import (
	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	bookkeepingrpc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/rpc/v1"
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	healthrpc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/rpc/v1"
	ledgersapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	ledgersrpc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/rpc/v1"
	platformrpc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// Modules are the use cases the methods call.
type Modules struct {
	Health      healthdto.Source
	Ledgers     ledgersapp.Application
	Bookkeeping bookkeepingapp.Application
}

// Router builds the router with every method.
func Router(m Modules) *platformrpc.Router {
	r := platformrpc.NewRouter()
	healthrpc.Register(r, m.Health)
	ledgersrpc.Register(r, m.Ledgers)
	bookkeepingrpc.Register(r, m.Bookkeeping)
	return r
}
