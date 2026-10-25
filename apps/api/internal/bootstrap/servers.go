package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	bookkeepingv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/rest/v1"
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	ledgersv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/rest/v1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcserver"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/httpserver"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	graphqltransport "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/graphql"
	grpctransport "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/grpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/rest"
)

// healthSource is what the health answers of every transport read.
func healthSource(r resources, cfg config.Config, svc Service) healthdto.Source {
	return healthdto.Source{
		Readiness: r.readiness,
		Database:  r.db,
		Service: healthdto.Service{
			Name:                     svc.Name,
			Version:                  svc.Version,
			StartedAt:                time.Now().UTC(),
			TigerBeetleClientVersion: moduleVersion("github.com/tigerbeetle/tigerbeetle-go"),
			TigerBeetleClusterID:     clusterID(cfg.TigerBeetleClusterID),
		},
	}
}

// httpHandler builds the HTTP handler: the health operations always, the module operations when
// REST is enabled, and POST /graphql when GraphQL is, on one ServeMux behind the standard
// middleware of platform/restapi (request ID, access log, panic recovery).
func httpHandler(health healthdto.Source, m modules, cfg config.Config, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	api := restapi.NewAPI(mux)
	rest.RegisterHealth(api, health)
	if cfg.RESTEnabled {
		rest.RegisterModules(api, rest.Handlers{
			Ledgers: ledgersv1.Deps{
				LedgerCommands:   m.ledgers.LedgerCommands,
				LedgerQueries:    m.ledgers.LedgerQueries,
				SettingsCommands: m.ledgers.SettingsCommands,
				SettingsQueries:  m.ledgers.SettingsQueries,
			},
			Bookkeeping: bookkeepingv1.Deps{
				TransactionCommands: m.bookkeeping.TransactionCommands,
				TransactionQueries:  m.bookkeeping.TransactionQueries,
				HoldCommands:        m.bookkeeping.HoldCommands,
				HoldQueries:         m.bookkeeping.HoldQueries,
				AccountCommands:     m.bookkeeping.AccountCommands,
				AccountQueries:      m.bookkeeping.AccountQueries,
				RequestQueries:      m.bookkeeping.RequestQueries,
				OverviewQueries:     m.bookkeeping.OverviewQueries,
				Reconciler:          m.bookkeeping.Reconciler,
			},
		})
	}
	if cfg.GraphQLEnabled {
		mux.Handle("POST /graphql", graphqltransport.NewHandler(graphqltransport.NewResolver(graphqltransport.Modules{
			Health:      health,
			Ledgers:     m.ledgers,
			Bookkeeping: m.bookkeeping,
		})))
	}
	return restapi.Handler(mux, logger)
}

// grpcServer builds the gRPC server with every module's services and server reflection.
func grpcServer(health healthdto.Source, m modules, cfg config.Config, logger *slog.Logger) *grpcserver.Server {
	srv := grpcserver.New(grpcserver.Options{
		Addr: cfg.GRPCAddr, ShutdownTimeout: cfg.ShutdownTimeout, Logger: logger, Reflection: true,
	})
	grpctransport.RegisterAll(srv, grpctransport.Modules{Health: health, Ledgers: m.ledgers, Bookkeeping: m.bookkeeping})
	return srv
}

// runners lists the enabled servers and workers.
func runners(cfg config.Config, a *App, logger *slog.Logger) []Runner {
	out := []Runner{httpRunner{cfg: cfg, handler: a.handler, logger: logger}}
	if a.grpc != nil {
		out = append(out, grpcRunner{srv: a.grpc})
	}
	if cfg.ReconcilerEnabled {
		out = append(out, reconcilerRunner{a: a})
	}
	return out
}

// httpRunner serves REST and the health endpoints.
type httpRunner struct {
	cfg     config.Config
	handler http.Handler
	logger  *slog.Logger
}

func (httpRunner) Name() string { return "http" }

func (h httpRunner) Run(ctx context.Context) error {
	return httpserver.Run(ctx, httpserver.Options{
		Addr:            h.cfg.Addr,
		Handler:         h.handler,
		ShutdownTimeout: h.cfg.ShutdownTimeout,
		Logger:          h.logger,
	})
}

// grpcRunner serves gRPC.
type grpcRunner struct {
	srv *grpcserver.Server
}

func (grpcRunner) Name() string { return "grpc" }

func (g grpcRunner) Run(ctx context.Context) error {
	return g.srv.Run(ctx)
}

// reconcilerRunner settles writes interrupted by a crash or a TigerBeetle timeout (ADR-0007).
type reconcilerRunner struct {
	a *App
}

func (reconcilerRunner) Name() string { return "reconciler" }

func (r reconcilerRunner) Run(ctx context.Context) error {
	r.a.reconciler.Run(ctx)
	return nil
}

// moduleVersion reads the version of a dependency from the build information.
func moduleVersion(path string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == path {
			if dep.Replace != nil {
				return dep.Replace.Version
			}
			return dep.Version
		}
	}
	return "unknown"
}

// clusterID renders the TigerBeetle cluster ID, a u128 kept as a decimal string.
func clusterID(id uint64) string {
	return strconv.FormatUint(id, 10)
}
