package grpcserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn/authntest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
)

// probeService is a hand-built service for the interceptors: no proto file is needed to exercise
// them. Each method name is what its handler does.
type probeService struct {
	release chan struct{}
}

func unary(name string, call func(ctx context.Context) (any, error)) grpc.MethodDesc {
	return grpc.MethodDesc{
		MethodName: name,
		Handler: func(_ any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			if err := dec(new(emptypb.Empty)); err != nil {
				return nil, err
			}
			info := &grpc.UnaryServerInfo{FullMethod: "/probe.Probe/" + name}
			return interceptor(ctx, new(emptypb.Empty), info, func(ctx context.Context, _ any) (any, error) { return call(ctx) })
		},
	}
}

func (p *probeService) desc() *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: "probe.Probe",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			unary("RequestID", func(ctx context.Context) (any, error) { return wrapperspb.String(requestid.From(ctx)), nil }),
			unary("Fail", func(context.Context) (any, error) {
				return nil, ledger.NewProblem(ledger.ErrRejected, "exceeds-credits", "not enough")
			}),
			unary("Panic", func(context.Context) (any, error) { panic("secret panic text") }),
			unary("Block", func(ctx context.Context) (any, error) {
				select {
				case <-p.release:
					return new(emptypb.Empty), nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}),
		},
	}
}

func newTestServer(t *testing.T, timeout time.Duration) (*Server, *bufconn.Listener, *probeService) {
	t.Helper()
	srv := New(Options{ShutdownTimeout: timeout, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Auth: authntest.Default()})
	probe := &probeService{release: make(chan struct{})}
	srv.RegisterService(probe.desc(), probe)
	return srv, bufconn.Listen(1 << 20), probe
}

func dial(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// dialAddr connects in plaintext to a server on a real listener.
func dialAddr(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestInterceptors(t *testing.T) {
	t.Parallel()
	srv, lis, _ := newTestServer(t, time.Second)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn := dial(t, lis)
	ctx := t.Context()

	t.Run("keeps a valid request ID and echoes it", func(t *testing.T) {
		var header metadata.MD
		out := new(wrapperspb.StringValue)
		callCtx := metadata.AppendToOutgoingContext(ctx, RequestIDKey, "client-id-1")
		if err := conn.Invoke(callCtx, "/probe.Probe/RequestID", new(emptypb.Empty), out, grpc.Header(&header)); err != nil {
			t.Fatal(err)
		}
		if out.GetValue() != "client-id-1" || headerID(header) != "client-id-1" {
			t.Fatalf("handler saw %q, header %q", out.GetValue(), headerID(header))
		}
	})

	t.Run("replaces an invalid request ID", func(t *testing.T) {
		var header metadata.MD
		out := new(wrapperspb.StringValue)
		callCtx := metadata.AppendToOutgoingContext(ctx, RequestIDKey, "has space")
		if err := conn.Invoke(callCtx, "/probe.Probe/RequestID", new(emptypb.Empty), out, grpc.Header(&header)); err != nil {
			t.Fatal(err)
		}
		if !requestid.Valid(out.GetValue()) || out.GetValue() == "has space" || headerID(header) != out.GetValue() {
			t.Fatalf("handler saw %q, header %q", out.GetValue(), headerID(header))
		}
	})

	t.Run("maps a domain error with its details", func(t *testing.T) {
		var header metadata.MD
		err := conn.Invoke(ctx, "/probe.Probe/Fail", new(emptypb.Empty), new(emptypb.Empty), grpc.Header(&header))
		st := status.Convert(err)
		info, _, _ := detailsOf(t, st)
		if st.Code() != codes.FailedPrecondition || info.GetReason() != "EXCEEDS_CREDITS" {
			t.Fatalf("status = %v, ErrorInfo = %v", st, info)
		}
		if info.GetMetadata()["requestId"] != headerID(header) {
			t.Errorf("ErrorInfo requestId %q, header %q", info.GetMetadata()["requestId"], headerID(header))
		}
	})

	t.Run("a panic is an internal error with no text", func(t *testing.T) {
		err := conn.Invoke(ctx, "/probe.Probe/Panic", new(emptypb.Empty), new(emptypb.Empty))
		st := status.Convert(err)
		if st.Code() != codes.Internal || st.Message() != internalMessage {
			t.Fatalf("status = %v", st)
		}
	})

	t.Run("an unknown method is Unimplemented", func(t *testing.T) {
		err := conn.Invoke(ctx, "/probe.Probe/Nope", new(emptypb.Empty), new(emptypb.Empty))
		if status.Code(err) != codes.Unimplemented {
			t.Fatalf("err = %v", err)
		}
	})
}

// TestDrain checks the shutdown: a call in flight finishes when it can, and one that never ends is
// cut off at the deadline instead of holding the process.
func TestDrain(t *testing.T) {
	t.Parallel()

	t.Run("waits for a call that finishes", func(t *testing.T) {
		t.Parallel()
		srv, lis, probe := newTestServer(t, 5*time.Second)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- srv.serveUntil(ctx, lis) }()
		conn := dial(t, lis)
		callErr := make(chan error, 1)
		go func() {
			callErr <- conn.Invoke(t.Context(), "/probe.Probe/Block", new(emptypb.Empty), new(emptypb.Empty))
		}()
		time.Sleep(100 * time.Millisecond)
		cancel()
		time.Sleep(100 * time.Millisecond)
		close(probe.release)
		if err := <-callErr; err != nil {
			t.Fatalf("call in flight failed: %v", err)
		}
		if err := <-done; err != nil {
			t.Fatalf("serveUntil = %v", err)
		}
	})

	t.Run("cuts off a call that never finishes", func(t *testing.T) {
		t.Parallel()
		srv, lis, _ := newTestServer(t, 200*time.Millisecond)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- srv.serveUntil(ctx, lis) }()
		conn := dial(t, lis)
		callErr := make(chan error, 1)
		go func() {
			callErr <- conn.Invoke(t.Context(), "/probe.Probe/Block", new(emptypb.Empty), new(emptypb.Empty))
		}()
		time.Sleep(100 * time.Millisecond)
		start := time.Now()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("serveUntil = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("serveUntil did not return after the shutdown timeout")
		}
		if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
			t.Errorf("returned after %v, before the shutdown timeout", elapsed)
		}
		if err := <-callErr; err == nil {
			t.Fatal("the call in flight succeeded, want it cut off")
		}
	})
}

func headerID(md metadata.MD) string {
	if v := md.Get(RequestIDKey); len(v) > 0 {
		return v[0]
	}
	return ""
}
