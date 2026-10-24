// Command api is the platform API server. main wires every dependency by hand: no injection
// framework, no globals, no init side effects.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/tigerbeetle"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/httpserver"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
)

const serviceName = "tigerbeetle-platform-api"

// version is overridden at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("api", flag.ContinueOnError)
	flags.SetOutput(stdout)
	showVersion := flags.Bool("version", false, "print the version and exit")
	probeURL := flags.String("probe", "", "GET this URL, exit 0 on a 2xx answer (container healthcheck)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintf(stdout, "%s %s\n", serviceName, version)
		return err
	}
	if *probeURL != "" {
		return probe(context.Background(), *probeURL, 5*time.Second)
	}

	// SIGINT and SIGTERM cancel ctx; everything below drains in reverse order of construction.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	level, err := cfg.Level()
	if err != nil {
		return err
	}
	logger := logging.New(stdout, level).With(slog.String("service", serviceName), slog.String("version", version))

	metadata, err := postgres.New(ctx, cfg.PostgresURL)
	if err != nil {
		return err
	}
	defer metadata.Close()

	if cfg.MigrateOnStart {
		migrateCtx, cancel := context.WithTimeout(ctx, time.Minute)
		err := metadata.Migrate(migrateCtx, logger)
		cancel()
		if err != nil {
			return err
		}
	}

	// The TigerBeetle client wants IP addresses; compose and Kubernetes hand us names.
	addresses, err := tigerbeetle.ResolveAddresses(ctx, cfg.TigerBeetleAddresses)
	if err != nil {
		return err
	}
	ledgerStore, err := tigerbeetle.New(cfg.TigerBeetleClusterID, addresses)
	if err != nil {
		return err
	}
	defer ledgerStore.Close()

	status := app.NewStatus(ledgerStore, metadata, cfg.ReadinessTimeout)

	handler := httpapi.NewHandler(httpapi.Deps{
		Logger:  logger,
		Status:  status,
		Service: serviceName,
		Version: version,
	})

	err = httpserver.Run(ctx, httpserver.Options{
		Addr:            cfg.Addr,
		Handler:         handler,
		ShutdownTimeout: cfg.ShutdownTimeout,
		Logger:          logger,
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	logger.InfoContext(ctx, "stopped")
	return nil
}
