// Package grpc registers every module's gRPC services on one server (ADR-0021), the gRPC
// counterpart of internal/transport/rest. Two services span modules, as their REST tags do:
// LedgerService (ledgers, plus bookkeeping's assets, stats and search) and OperationsService
// (health's info, plus bookkeeping's reconciler). Joining them here keeps the modules apart.
package grpc

import (
	grpclib "google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	bookkeepinggrpc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/grpc/v1"
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	healthgrpc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/grpc/v1"
	ledgersapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	ledgersgrpc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/grpc/v1"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

// Modules are the use cases the services call.
type Modules struct {
	Health      healthdto.Source
	Ledgers     ledgersapp.Application
	Bookkeeping bookkeepingapp.Application
}

// RegisterAll registers every service.
func RegisterAll(r grpclib.ServiceRegistrar, m Modules) {
	pb.RegisterLedgerServiceServer(r, ledgerService{
		LedgerServer:   ledgersgrpc.NewLedgerServer(m.Ledgers),
		OverviewServer: bookkeepinggrpc.NewOverviewServer(m.Bookkeeping),
	})
	pb.RegisterSettingsServiceServer(r, ledgersgrpc.NewSettingsServer(m.Ledgers))
	pb.RegisterAccountServiceServer(r, bookkeepinggrpc.NewAccountServer(m.Bookkeeping))
	pb.RegisterTransactionServiceServer(r, bookkeepinggrpc.NewTransactionServer(m.Bookkeeping))
	pb.RegisterHoldServiceServer(r, bookkeepinggrpc.NewHoldServer(m.Bookkeeping))
	pb.RegisterWriteRequestServiceServer(r, bookkeepinggrpc.NewWriteRequestServer(m.Bookkeeping))
	pb.RegisterOperationsServiceServer(r, operationsService{
		InfoServer:       healthgrpc.NewInfoServer(m.Health),
		ReconcilerServer: bookkeepinggrpc.NewReconcilerServer(m.Bookkeeping),
	})
	healthpb.RegisterHealthServer(r, healthgrpc.NewHealthServer(m.Health))
}

// ledgerService joins the two halves of LedgerService. The stubs are generated with
// require_unimplemented_servers=false, so a method neither half has is a compile error here.
type ledgerService struct {
	*ledgersgrpc.LedgerServer
	*bookkeepinggrpc.OverviewServer
}

// operationsService joins the two halves of OperationsService.
type operationsService struct {
	*healthgrpc.InfoServer
	*bookkeepinggrpc.ReconcilerServer
}
