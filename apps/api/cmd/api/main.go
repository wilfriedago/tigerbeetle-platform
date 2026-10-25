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
	"sync"
	"syscall"
	"time"

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
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
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

	svc, err := wire(ctx, cfg, logger)
	if err != nil {
		return err
	}

	defer svc.close()

	// The reconciler settles writes interrupted by a crash or a TigerBeetle timeout. It stops with
	// ctx and is waited for before the stores close (defers run in reverse order).
	var background sync.WaitGroup
	background.Go(func() { svc.reconciler.Run(ctx) })
	defer background.Wait()

	err = httpserver.Run(ctx, httpserver.Options{
		Addr:            cfg.Addr,
		Handler:         svc.handler,
		ShutdownTimeout: cfg.ShutdownTimeout,
		Logger:          logger,
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	logger.InfoContext(ctx, "stopped")

	return nil
}
