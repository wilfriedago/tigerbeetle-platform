package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi"
	v1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/v1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/httpserver"
)

// restHandler builds the HTTP handler of the REST operations.
func restHandler(r resources, m modules, cfg config.Config, svc Service, logger *slog.Logger) http.Handler {
	return httpapi.NewHandler(logger, v1.Deps{
		Readiness:    r.readiness,
		Database:     r.db,
		Ledgers:      m.ledgers,
		Settings:     m.settings,
		Accounts:     m.accounts,
		Transactions: m.transactions,
		Holds:        m.holds,
		Overview:     m.overview,
		Reconciler:   m.reconciler,
		Service: v1.Service{
			Name:                     svc.Name,
			Version:                  svc.Version,
			StartedAt:                time.Now().UTC(),
			TigerBeetleClientVersion: moduleVersion("github.com/tigerbeetle/tigerbeetle-go"),
			TigerBeetleClusterID:     clusterID(cfg.TigerBeetleClusterID),
		},
	})
}

// runners lists the enabled servers and workers.
func runners(cfg config.Config, a *App, logger *slog.Logger) []Runner {
	out := []Runner{httpRunner{cfg: cfg, handler: a.handler, logger: logger}}
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
