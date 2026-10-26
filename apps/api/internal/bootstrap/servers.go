package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	accessv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/rest/v1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/web"
	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	bookkeepingkafka "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/kafka"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/cdc"
	bookkeepingv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/rest/v1"
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	ledgersv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/rest/v1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/amqprpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcserver"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/httpserver"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/kafka"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/natsrpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/readiness"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
	graphqltransport "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/graphql"
	grpctransport "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/grpc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/rest"
	rpctransport "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/rpc"
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
func httpHandler(health healthdto.Source, m modules, cfg config.Config, logger *slog.Logger, auth *authn.Authenticator, signIn *console) http.Handler {
	mux := http.NewServeMux()
	api := restapi.NewAPI(mux)
	rest.RegisterHealth(api, health)
	if cfg.RESTEnabled {
		rest.RegisterModules(api, rest.Handlers{
			Access: accessv1.Deps{
				PlatformOrganizations: m.access.PlatformOrganizations,
				Members:               m.access.Members,
				Credentials:           m.access.Credentials,
			},
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
				Reconciler:          m.bookkeeping.ReconcilerQueries,
			},
		})
	}
	if cfg.GraphQLEnabled {
		mux.Handle("POST /graphql", graphqltransport.NewHandler(graphqltransport.NewResolver(graphqltransport.Modules{
			Access:      m.access,
			Health:      health,
			Ledgers:     m.ledgers,
			Bookkeeping: m.bookkeeping,
		})))
	}
	// Liveness and readiness are for orchestrators and load balancers: no credential.
	opts := restapi.Options{Auth: auth, Public: []string{"/healthz", "/readyz"}}
	if signIn != nil {
		web.Register(mux, signIn.sessions, m.access.ViewerQueries, signIn.web)
		opts.Public = append(opts.Public, web.Paths...)
		opts.Sessions, opts.SessionCookie, opts.ConsoleOrigin = signIn.sessions, signIn.web.SessionCookie(), signIn.web.ConsoleOrigin
	}
	return restapi.Handler(mux, logger, opts)
}

// grpcServer builds the gRPC server with every module's services and server reflection.
func grpcServer(health healthdto.Source, m modules, cfg config.Config, logger *slog.Logger, auth *authn.Authenticator) *grpcserver.Server {
	srv := grpcserver.New(grpcserver.Options{
		Addr: cfg.GRPCAddr, ShutdownTimeout: cfg.ShutdownTimeout, Logger: logger, Reflection: true, Auth: auth,
	})
	grpctransport.RegisterAll(srv, grpctransport.Modules{Health: health, Ledgers: m.ledgers, Bookkeeping: m.bookkeeping})
	return srv
}

// brokerServers builds what talks to a broker: the AMQP and NATS RPC servers on one router and
// dispatcher, so both answer every method the same way, and the Kafka relay. Their readiness
// probes are optional: a broker outage degrades the service without making it unready (plan
// section 13.2). RabbitMQ is one dependency, reported once, even when both AMQP RPC and the relay
// connect to it.
func (a *App) brokerServers(r resources, health healthdto.Source, m modules, cfg config.Config, logger *slog.Logger, auth *authn.Authenticator) error {
	var rabbit []readiness.Probe
	defer func() {
		if len(rabbit) > 0 {
			r.readiness.Optional(healthdto.ProbeRabbitMQ, allOf(rabbit))
		}
	}()
	if cfg.KafkaEventsEnabled {
		probe, err := a.kafkaRelay(r, m, cfg, logger)
		if err != nil {
			return err
		}
		rabbit = append(rabbit, probe)
	}
	if !cfg.AMQPRPCEnabled && !cfg.NATSRPCEnabled {
		return nil
	}
	router := rpctransport.Router(rpctransport.Modules{Health: health, Ledgers: m.ledgers, Bookkeeping: m.bookkeeping})
	dispatcher := rpc.NewDispatcher(router, cfg.RPCTimeoutMax, logger, auth, "v1.getLiveness", "v1.getReadiness")
	if cfg.AMQPRPCEnabled {
		a.amqp = amqprpc.New(amqprpc.Options{
			URL: cfg.AMQPURL, Queue: cfg.AMQPRPCQueue, Concurrency: cfg.RPCConcurrency,
			ShutdownTimeout: cfg.ShutdownTimeout, Dispatcher: dispatcher, Logger: logger,
		})
		rabbit = append(rabbit, a.amqp.Ready)
	}
	if cfg.NATSRPCEnabled {
		srv, err := natsrpc.New(natsrpc.Options{
			URL: cfg.NATSURL, QueueGroup: cfg.NATSRPCQueueGroup, Concurrency: cfg.RPCConcurrency,
			ShutdownTimeout: cfg.ShutdownTimeout, Router: router, Dispatcher: dispatcher, Logger: logger,
		})
		if err != nil {
			return err
		}
		a.closers = append(a.closers, srv.Close)
		a.nats = srv
		r.readiness.Optional(healthdto.ProbeNATS, srv.Ready)
	}
	return nil
}

// kafkaRelay builds the ledger event relay (ADR-0022): the Kafka producer, the publisher, the
// relay use case and the CDC consumer. It registers the Kafka probe and returns the RabbitMQ one.
func (a *App) kafkaRelay(r resources, m modules, cfg config.Config, logger *slog.Logger) (readiness.Probe, error) {
	producer, err := kafka.New(cfg.KafkaBrokers, "tbp-api", 30*time.Second)
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, producer.Close)
	r.readiness.Optional(healthdto.ProbeKafka, producer.Ping)
	relay := bookkeepingapp.NewLedgerEventRelay(m.bookkeepingAccounts, bookkeepingkafka.NewEventPublisher(producer, cfg.KafkaEventsTopic))
	a.relay = cdc.NewConsumer(cdc.Options{URL: cfg.CDCAMQPURL, Queue: cfg.CDCQueue, Relay: relay, Logger: logger})
	return a.relay.Ready, nil
}

// allOf is a probe that passes when every probe passes.
func allOf(probes []readiness.Probe) readiness.Probe {
	return func(ctx context.Context) error {
		for _, p := range probes {
			if err := p(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// runners lists the enabled servers and workers.
func runners(cfg config.Config, a *App, logger *slog.Logger) []Runner {
	out := []Runner{httpRunner{cfg: cfg, handler: a.handler, logger: logger}}
	if a.grpc != nil {
		out = append(out, grpcRunner{srv: a.grpc})
	}
	if a.amqp != nil {
		out = append(out, funcRunner{name: "amqp-rpc", run: a.amqp.Run})
	}
	if a.nats != nil {
		out = append(out, funcRunner{name: "nats-rpc", run: a.nats.Run})
	}
	if a.relay != nil {
		out = append(out, funcRunner{name: "cdc-relay", run: a.relay.Run})
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

// funcRunner runs a server whose Run already has the Runner shape.
type funcRunner struct {
	name string
	run  func(context.Context) error
}

func (f funcRunner) Name() string { return f.name }

func (f funcRunner) Run(ctx context.Context) error { return f.run(ctx) }

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
