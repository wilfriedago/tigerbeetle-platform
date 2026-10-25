// Command api is the platform API server. main reads the configuration and hands it to
// internal/bootstrap, which wires every dependency by hand: no injection framework, no globals, no
// init side effects.
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

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/bootstrap"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
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

	// SIGINT and SIGTERM cancel ctx.
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

	app, err := bootstrap.Build(ctx, cfg, bootstrap.Service{Name: serviceName, Version: version}, logger)
	if err != nil {
		return err
	}

	// Run serves until SIGINT or SIGTERM, then drains the servers and closes the resources.
	if err := app.Run(ctx); err != nil {
		return err
	}

	logger.InfoContext(ctx, "stopped")

	return nil
}
