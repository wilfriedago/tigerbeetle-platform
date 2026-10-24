// Package httpserver runs an http.Server until the context is cancelled, then drains it.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Options configures Run.
type Options struct {
	Addr            string
	Handler         http.Handler
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

// Run serves until ctx is cancelled, then gives in-flight requests ShutdownTimeout to finish.
// It returns nil on a clean shutdown and the listen error otherwise.
func Run(ctx context.Context, opts Options) error {
	srv := &http.Server{
		Addr:              opts.Addr,
		Handler:           opts.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", opts.Addr)
	if err != nil {
		return fmt.Errorf("httpserver: listen %s: %w", opts.Addr, err)
	}
	opts.Logger.InfoContext(ctx, "http server listening", slog.String("addr", listener.Addr().String()))

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(listener)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("httpserver: serve: %w", err)
	case <-ctx.Done():
	}

	opts.Logger.InfoContext(ctx, "http server shutting down", slog.Duration("timeout", opts.ShutdownTimeout))
	// The parent context is already cancelled, so the drain needs its own deadline.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opts.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("httpserver: shutdown: %w", err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpserver: serve: %w", err)
	}
	return nil
}
