// Package grpcserver runs the gRPC server of the process (ADR-0021, plan section 11.1): one
// listener, interceptors for the request ID, the access log, panics and errors, and a drain on
// shutdown that gives up after a deadline instead of waiting forever.
package grpcserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc/credentials"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Options configures a Server.
type Options struct {
	Addr            string
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
	// Reflection lets grpcurl and similar tools list and describe the services.
	Reflection bool
	// Auth authenticates every call except health and reflection (ADR-0032).
	Auth Authenticator
	// TLS serves TLS with its certificate; nil serves plaintext, for a TLS-terminating proxy.
	TLS *tls.Config
}

// Server is a grpc.Server with the platform's interceptors. Services register on it before Run.
type Server struct {
	srv  *grpc.Server
	opts Options
}

// New builds a server. Interceptors run in this order, outermost first: request ID, access log,
// error mapping, panic recovery, authentication; so a panic becomes an error, a refused credential
// and any other error become a status with details, and the log line sees the final code.
func New(opts Options) *Server {
	var serverOpts []grpc.ServerOption
	if opts.TLS != nil {
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(opts.TLS)))
	}
	srv := grpc.NewServer(append(serverOpts,
		grpc.ChainUnaryInterceptor(
			unaryRequestID(opts.Logger), unaryAccessLog, unaryErrors, unaryRecover, unaryAuth(opts.Auth),
		),
		grpc.ChainStreamInterceptor(
			streamRequestID(opts.Logger), streamAccessLog, streamErrors, streamRecover, streamAuth(opts.Auth),
		),
	)...)
	return &Server{srv: srv, opts: opts}
}

// RegisterService registers a service implementation, so a Server is a grpc.ServiceRegistrar.
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl any) {
	s.srv.RegisterService(desc, impl)
}

// Serve serves on lis until Stop or GracefulStop. Tests call it with an in-memory listener; Run
// is the production entry point.
func (s *Server) Serve(lis net.Listener) error {
	if s.opts.Reflection {
		reflection.Register(s.srv)
	}
	return s.srv.Serve(lis)
}

// Stop closes every connection at once.
func (s *Server) Stop() { s.srv.Stop() }

// Run listens on Addr and serves until ctx is cancelled, then lets calls in flight finish within
// ShutdownTimeout and closes what is left. It returns nil on a clean stop.
func (s *Server) Run(ctx context.Context) error {
	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", s.opts.Addr)
	if err != nil {
		return fmt.Errorf("grpcserver: listen %s: %w", s.opts.Addr, err)
	}
	s.opts.Logger.InfoContext(ctx, "grpc server listening", slog.String("addr", lis.Addr().String()))
	return s.serveUntil(ctx, lis)
}

// serveUntil serves on lis until ctx is cancelled, then drains.
func (s *Server) serveUntil(ctx context.Context, lis net.Listener) error {
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(lis) }()

	select {
	case err := <-errCh:
		return fmt.Errorf("grpcserver: serve: %w", err)
	case <-ctx.Done():
	}

	s.opts.Logger.InfoContext(ctx, "grpc server shutting down", slog.Duration("timeout", s.opts.ShutdownTimeout))
	stopped := make(chan struct{})
	go func() {
		s.srv.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(s.opts.ShutdownTimeout)
	defer timer.Stop()
	select {
	case <-stopped:
	case <-timer.C:
		// GracefulStop waits for every call and stream; a client that never finishes would hold
		// the process forever.
		s.opts.Logger.WarnContext(ctx, "grpc drain timed out, closing open calls")
		s.srv.Stop()
		<-stopped
	}
	if err := <-errCh; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("grpcserver: serve: %w", err)
	}
	return nil
}
