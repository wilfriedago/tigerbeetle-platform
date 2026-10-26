// Package bootstrap builds the application from its configuration and runs it: it opens the
// process-wide resources, builds the use cases in dependency order, and starts every enabled
// server and worker together (plan section 11.3). It is the only package that sees every concrete
// type, and it holds no business rule. cmd/api and the end-to-end tests both build through it, so
// the tests run exactly what the binary runs.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/cdc"
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/amqprpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcserver"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/natsrpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/readiness"
	platformtb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/tigerbeetle"
)

// Service identifies the running binary in GET /v1/_info.
type Service struct {
	Name    string
	Version string
}

// App is the built application: its servers and workers, and the resources they share.
type App struct {
	handler http.Handler
	// grpc is nil when GRPC_ENABLED is off.
	grpc *grpcserver.Server
	// amqp, nats and relay are nil when their switch is off.
	amqp       *amqprpc.Server
	nats       *natsrpc.Server
	relay      *cdc.Consumer
	reconciler *bookkeepingapp.Reconciler
	runners    []Runner
	// closers release the resources in construction order; close runs them in reverse.
	closers []func()
}

// resources are the process-wide connections every module shares.
type resources struct {
	db          *platformpg.DB
	tigerbeetle *platformtb.Client
	readiness   *readiness.Checker
}

// Build opens the resources, builds the modules and prepares the enabled servers and workers. It
// starts nothing; Run does. On error, everything already opened is closed again.
func Build(ctx context.Context, cfg config.Config, svc Service, logger *slog.Logger, opts ...Option) (_ *App, err error) {
	o := newOptions(opts)
	a := &App{}
	defer func() {
		if err != nil {
			a.close()
		}
	}()

	r, err := a.open(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	m := buildModules(r, cfg, logger, o)
	auth, verifier, err := authenticator(ctx, cfg, m)
	if err != nil {
		return nil, err
	}
	signIn, err := consoleSignIn(cfg, verifier, m)
	if err != nil {
		return nil, err
	}
	health := healthSource(r, cfg, svc)
	a.handler = httpHandler(health, m, cfg, logger, auth, signIn)
	if cfg.GRPCEnabled {
		a.grpc = grpcServer(health, m, cfg, logger, auth)
	}
	if err := a.brokerServers(r, health, m, cfg, logger, auth); err != nil {
		return nil, err
	}
	a.reconciler = m.bookkeeping.Reconciler
	a.runners = runners(cfg, a, logger)
	return a, nil
}

// open connects to Postgres, migrates it when asked, then creates the TigerBeetle client.
func (a *App) open(ctx context.Context, cfg config.Config, logger *slog.Logger) (resources, error) {
	db, err := platformpg.Open(ctx, cfg.PostgresURL)
	if err != nil {
		return resources{}, err
	}
	a.closers = append(a.closers, db.Close)
	if cfg.MigrateOnStart {
		migrateCtx, cancel := context.WithTimeout(ctx, time.Minute)
		err := db.Migrate(migrateCtx, logger)
		cancel()
		if err != nil {
			return resources{}, err
		}
	}

	// The TigerBeetle client wants IP addresses; compose and Kubernetes hand us names.
	addresses, err := platformtb.ResolveAddresses(ctx, cfg.TigerBeetleAddresses)
	if err != nil {
		return resources{}, err
	}
	client, err := platformtb.New(cfg.TigerBeetleClusterID, addresses, platformtb.WithRequestTimeout(cfg.TigerBeetleTimeout))
	if err != nil {
		return resources{}, fmt.Errorf("tigerbeetle: %w", err)
	}
	a.closers = append(a.closers, client.Close)

	checker := readiness.New(cfg.ReadinessTimeout)
	checker.Require(healthdto.ProbePostgres, db.Ready)
	checker.Require(healthdto.ProbeTigerBeetle, client.Ready)
	return resources{db: db, tigerbeetle: client, readiness: checker}, nil
}

// authenticator reads the identity provider's discovery document and builds the authenticator
// every transport shares (ADR-0026). Authentication is always on: without an issuer the API does
// not start.
func authenticator(ctx context.Context, cfg config.Config, m modules) (*authn.Authenticator, *oidc.Verifier, error) {
	verifier, err := oidc.New(ctx, oidc.Config{
		IssuerURL:         cfg.AuthIssuerURL,
		DiscoveryURL:      cfg.AuthDiscoveryURL,
		Audience:          cfg.AuthAudience,
		StaffClaim:        cfg.AuthStaffClaim,
		StaffValue:        cfg.AuthStaffValue,
		OrganizationClaim: cfg.AuthOrganizationClaim,
	})
	if err != nil {
		return nil, nil, err
	}
	return authn.New(verifier, m.access.OrganizationQueries), verifier, nil
}

// Handler is the HTTP handler of the API, for tests that serve it with httptest.
func (a *App) Handler() http.Handler {
	return a.handler
}

// GRPC is the gRPC server, nil when GRPC_ENABLED is off. Tests serve it on an in-memory listener.
func (a *App) GRPC() *grpcserver.Server {
	return a.grpc
}

// ReconcileOnce runs one pass of the reconciler and returns how many records it settled. Run
// starts the loop; tests call this to settle records at a known time.
func (a *App) ReconcileOnce(ctx context.Context) int {
	return a.reconciler.RunOnce(ctx)
}

// Close releases the resources without running anything, for an App that was built but not run.
func (a *App) Close() {
	a.close()
}

func (a *App) close() {
	for i := len(a.closers) - 1; i >= 0; i-- {
		a.closers[i]()
	}
	a.closers = nil
}
