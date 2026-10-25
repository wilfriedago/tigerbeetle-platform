package main

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi"
	v1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/v1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/tigerbeetle"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
)

// service is the wired application: the HTTP handler and the background reconciler.
type service struct {
	handler    http.Handler
	reconciler *app.Reconciler
	// close releases the TigerBeetle client, then the Postgres pool.
	close func()
}

// wireOption adjusts the wiring. Only tests use it, to inject faults around TigerBeetle.
type wireOption func(*wiring)

type wiring struct {
	wrapLedgerStore func(app.LedgerStore) app.LedgerStore
}

// wire builds every dependency by hand: the stores, the use cases and the HTTP adapter. It is the
// composition root of main, shared with the end-to-end integration test so the test runs exactly
// what the binary runs.
func wire(ctx context.Context, cfg config.Config, logger *slog.Logger, opts ...wireOption) (*service, error) {
	w := wiring{wrapLedgerStore: func(s app.LedgerStore) app.LedgerStore { return s }}
	for _, opt := range opts {
		opt(&w)
	}

	metadata, err := postgres.New(ctx, cfg.PostgresURL)
	if err != nil {
		return nil, err
	}
	if cfg.MigrateOnStart {
		migrateCtx, cancel := context.WithTimeout(ctx, time.Minute)
		err := metadata.Migrate(migrateCtx, logger)
		cancel()
		if err != nil {
			metadata.Close()
			return nil, err
		}
	}

	// The TigerBeetle client wants IP addresses; compose and Kubernetes hand us names.
	addresses, err := tigerbeetle.ResolveAddresses(ctx, cfg.TigerBeetleAddresses)
	if err != nil {
		metadata.Close()
		return nil, err
	}
	ledgerStore, err := tigerbeetle.New(cfg.TigerBeetleClusterID, addresses, tigerbeetle.WithRequestTimeout(cfg.TigerBeetleTimeout))
	if err != nil {
		metadata.Close()
		return nil, err
	}

	clock := app.ClockFunc(time.Now)
	tb := w.wrapLedgerStore(ledgerStore)
	writes := app.TransactionsDeps{
		Store:       metadata,
		TigerBeetle: tb,
		IDs:         ledgerStore,
		Clock:       clock,
		GracePeriod: cfg.ReconcilerGracePeriod,
		Logger:      logger,
	}
	accounts := app.NewAccounts(metadata, metadata, metadata, tb, clock)
	transactions := app.NewTransactions(writes)
	holds := app.NewHolds(writes)
	reconciler := app.NewReconciler(metadata, transactions, holds, clock, cfg.ReconcilerGracePeriod, cfg.ReconcilerInterval, logger)

	handler := httpapi.NewHandler(logger, v1.Deps{
		Status:       app.NewStatus(ledgerStore, metadata, cfg.ReadinessTimeout),
		Ledgers:      app.NewLedgers(metadata, clock),
		Settings:     app.NewSettings(metadata, metadata, metadata, metadata, metadata, clock),
		Accounts:     accounts,
		Transactions: transactions,
		Holds:        holds,
		Overview:     app.NewOverview(metadata, accounts, tb, clock),
		Reconciler:   reconciler,
		Service: v1.Service{
			Name:                     serviceName,
			Version:                  version,
			StartedAt:                time.Now().UTC(),
			TigerBeetleClientVersion: moduleVersion("github.com/tigerbeetle/tigerbeetle-go"),
			TigerBeetleClusterID:     strconv.FormatUint(cfg.TigerBeetleClusterID, 10),
		},
	})

	return &service{
		handler:    handler,
		reconciler: reconciler,
		close: func() {
			ledgerStore.Close()
			metadata.Close()
		},
	}, nil
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
