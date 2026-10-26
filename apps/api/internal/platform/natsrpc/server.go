// Package natsrpc serves the broker RPC methods over NATS (ADR-0021, plan section 11.2): a micro
// service whose endpoints, tbp.rpc.v1.<operationId>, share one queue group, so NATS hands each
// request to one replica. micro calls a handler serially per endpoint, so the handler only takes a
// worker-pool slot and answers from that goroutine; the pool bounds concurrency. Core NATS keeps
// no request: a client retries a write with the same idempotency key.
package natsrpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// SubjectPrefix comes before the operationId in every endpoint subject.
const SubjectPrefix = "tbp.rpc.v1"

// The message headers, in the HTTP spelling; the server matches them without regard to case.
const (
	HeaderRequestID      = "X-Request-Id"
	HeaderIdempotencyKey = "Idempotency-Key"
	HeaderDeadline       = "X-Deadline"
	HeaderAuthorization  = "Authorization"
	HeaderOrganization   = "Organization"
)

// Options configures a Server.
type Options struct {
	URL string
	// QueueGroup is shared by every replica, tbp-api by default (NATS_RPC_QUEUE_GROUP).
	QueueGroup      string
	Concurrency     int
	ShutdownTimeout time.Duration
	Router          *rpc.Router
	Dispatcher      *rpc.Dispatcher
	Logger          *slog.Logger
	// TLS secures the connection; nil keeps the URL's scheme (tls:// turns TLS on).
	TLS *tls.Config
}

// Server is the micro service.
type Server struct {
	opts Options
	pool *rpc.Pool
	nc   *nats.Conn
}

// New connects and registers every method of the router as an endpoint. The connection keeps
// retrying, also when NATS is down at start-up; readiness reports it down meanwhile.
func New(opts Options) (*Server, error) {
	secure := func(*nats.Options) error { return nil }
	if opts.TLS != nil {
		secure = nats.Secure(opts.TLS)
	}
	nc, err := nats.Connect(opts.URL,
		secure,
		nats.Name("tbp-api rpc"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			opts.Logger.Warn("nats disconnected", slog.Any("error", err))
		}),
		nats.ReconnectHandler(func(*nats.Conn) { opts.Logger.Info("nats reconnected") }),
	)
	if err != nil {
		return nil, fmt.Errorf("natsrpc: connect: %w", err)
	}
	return &Server{opts: opts, pool: rpc.NewPool(opts.Concurrency), nc: nc}, nil
}

// Run serves until ctx ends, then stops the endpoints, lets the requests in flight finish within
// ShutdownTimeout and closes the connection.
func (s *Server) Run(ctx context.Context) error {
	defer s.nc.Close()
	svc, err := micro.AddService(s.nc, micro.Config{
		Name:        "tbp-api",
		Version:     "1.0.0",
		Description: "The tigerbeetle-platform API over NATS request and reply.",
		QueueGroup:  s.opts.QueueGroup,
	})
	if err != nil {
		return fmt.Errorf("natsrpc: add service: %w", err)
	}
	group := svc.AddGroup(SubjectPrefix)
	work := context.WithoutCancel(ctx)
	for _, method := range s.opts.Router.Methods() {
		handler := micro.HandlerFunc(func(req micro.Request) { s.accept(work, method, req) })
		if err := group.AddEndpoint(strings.TrimPrefix(method, "v1."), handler); err != nil {
			_ = svc.Stop()
			return fmt.Errorf("natsrpc: add endpoint %s: %w", method, err)
		}
	}
	s.opts.Logger.InfoContext(ctx, "nats rpc serving", slog.String("subjects", SubjectPrefix+".*"), slog.String("queue_group", s.opts.QueueGroup))

	<-ctx.Done()
	// Stop drains the subscriptions: requests already received are still handed to accept.
	if err := svc.Stop(); err != nil {
		s.opts.Logger.WarnContext(ctx, "nats rpc stop", slog.Any("error", err))
	}
	drain, cancel := context.WithTimeout(work, s.opts.ShutdownTimeout)
	defer cancel()
	if err := s.pool.Wait(drain); err != nil {
		s.opts.Logger.WarnContext(ctx, "nats rpc drain timed out; clients retry with their keys")
	}
	return nil
}

// Ready reports whether the connection is up and the server answers a flush.
func (s *Server) Ready(ctx context.Context) error {
	if !s.nc.IsConnected() {
		return errors.New("natsrpc: not connected")
	}
	// nats.go refuses a flush without a deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}
	return s.nc.FlushWithContext(ctx)
}

// accept runs on micro's goroutine for the endpoint: it waits for a pool slot and answers from
// the pool.
func (s *Server) accept(ctx context.Context, method string, req micro.Request) {
	call := rpc.Call{
		Method:         method,
		Payload:        req.Data(),
		RequestID:      headerOf(req.Headers(), HeaderRequestID),
		IdempotencyKey: headerOf(req.Headers(), HeaderIdempotencyKey),
		Deadline:       headerOf(req.Headers(), HeaderDeadline),
		Authorization:  headerOf(req.Headers(), HeaderAuthorization),
		Organization:   headerOf(req.Headers(), HeaderOrganization),
	}
	_ = s.pool.Go(ctx, func() { s.answer(ctx, call, req) })
}

// answer runs on the pool, after micro's handler has returned. It publishes the reply itself
// rather than through req.Respond or req.Error: those record the outcome on the request, which
// micro reads for its statistics once the handler returns, a data race once the answer comes from
// another goroutine. micro's error count therefore leaves these replies out.
func (s *Server) answer(ctx context.Context, call rpc.Call, req micro.Request) {
	answer := s.opts.Dispatcher.Answer(ctx, call)
	reply := &nats.Msg{Subject: req.Reply(), Data: answer.Body, Header: nats.Header{HeaderRequestID: []string{answer.RequestID}}}
	if answer.Error != nil {
		// micro's error headers carry the REST status and the reason, so `nats micro` tooling
		// shows them; the body is still the envelope.
		description := answer.Error.Reason
		if description == "" {
			description = answer.Error.Title
		}
		reply.Header.Set(micro.ErrorCodeHeader, strconv.Itoa(answer.Error.Status))
		reply.Header.Set(micro.ErrorHeader, description)
	}
	var err error
	if reply.Subject == "" {
		err = errors.New("no reply subject")
	} else {
		err = s.nc.PublishMsg(reply)
	}
	if err != nil {
		s.opts.Logger.WarnContext(ctx, "nats rpc reply not sent", slog.String("request_id", answer.RequestID), slog.Any("error", err))
	}
}

// headerOf reads a header without regard to case: NATS keeps the spelling the client sent.
func headerOf(h micro.Headers, key string) string {
	if v := nats.Header(h).Get(key); v != "" {
		return v
	}
	for k, v := range h {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// Close closes the connection, for a server built but never run. Closing twice is harmless.
func (s *Server) Close() { s.nc.Close() }
