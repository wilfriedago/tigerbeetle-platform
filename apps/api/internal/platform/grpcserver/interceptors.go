package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
)

// RequestIDKey is the metadata key that carries the request ID in and out, the gRPC spelling of
// the X-Request-Id header (metadata keys are lowercase).
const RequestIDKey = "x-request-id"

// incomingRequestID keeps a valid client-supplied request ID or makes one, and derives a logger
// that carries it, as the HTTP middleware does.
func incomingRequestID(ctx context.Context, base *slog.Logger) (out context.Context, id string) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get(RequestIDKey); len(v) > 0 {
			id = v[0]
		}
	}
	if !requestid.Valid(id) {
		id = requestid.New()
	}
	ctx = requestid.With(ctx, id)
	// Every call acts for the default organization until authentication resolves the tenant from
	// the credential (docs/plans/auth/plan.md, phase 2; ADR-0026).
	ctx = ledger.WithTenant(ctx, ledger.DefaultTenant)
	return logging.With(ctx, base.With(slog.String("request_id", id))), id
}

func unaryRequestID(base *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, id := incomingRequestID(ctx, base)
		// The header goes out with the response, success or error.
		_ = grpc.SetHeader(ctx, metadata.Pairs(RequestIDKey, id))
		return handler(ctx, req)
	}
}

func streamRequestID(base *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, id := incomingRequestID(ss.Context(), base)
		_ = ss.SetHeader(metadata.Pairs(RequestIDKey, id))
		return handler(srv, &contextStream{ServerStream: ss, ctx: ctx})
	}
}

// contextStream replaces the context of a stream.
type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextStream) Context() context.Context { return s.ctx }

func unaryAccessLog(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	logCall(ctx, info.FullMethod, err, start)
	return resp, err
}

func streamAccessLog(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	logCall(ss.Context(), info.FullMethod, err, start)
	return err
}

// logCall writes one line per call, like the HTTP access log.
func logCall(ctx context.Context, method string, err error, start time.Time) {
	logging.From(ctx).InfoContext(ctx, "request",
		slog.String("method", method),
		slog.String("code", status.Code(err).String()),
		slog.Duration("duration", time.Since(start)),
	)
}

func unaryErrors(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return resp, nil
}

func streamErrors(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := handler(srv, ss); err != nil {
		return toStatus(ss.Context(), err)
	}
	return nil
}

// toStatus maps an error to the status the client sees and logs the cause: at error level when
// the fault is ours, at info level when the request was refused. The cause's text goes to the log
// only.
func toStatus(ctx context.Context, err error) error {
	st := Status(err, requestid.From(ctx))
	logger := logging.From(ctx)
	if serverFault(st.Code()) {
		logger.ErrorContext(ctx, "request failed", slog.String("code", st.Code().String()), slog.Any("error", err))
	} else {
		logger.InfoContext(ctx, "request rejected", slog.String("code", st.Code().String()), slog.Any("error", err))
	}
	return st.Err()
}

func serverFault(c codes.Code) bool {
	return c == codes.Internal || c == codes.Unknown || c == codes.Unavailable || c == codes.DataLoss
}

// errPanic is what a recovered panic becomes; its text never reaches the client.
type errPanic struct{ value any }

func (e errPanic) Error() string { return fmt.Sprintf("panic: %v", e.value) }

func unaryRecover(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			logPanic(ctx, rec)
			resp, err = nil, errPanic{rec}
		}
	}()
	return handler(ctx, req)
}

func streamRecover(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			logPanic(ss.Context(), rec)
			err = errPanic{rec}
		}
	}()
	return handler(srv, ss)
}

func logPanic(ctx context.Context, rec any) {
	logging.From(ctx).ErrorContext(ctx, "panic", slog.Any("panic", rec), slog.String("stack", string(debug.Stack())))
}
