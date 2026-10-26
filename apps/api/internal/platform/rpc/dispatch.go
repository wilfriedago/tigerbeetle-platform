package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
)

// The message headers both brokers read. NATS headers are matched without regard to case.
const (
	HeaderRequestID      = "x-request-id"
	HeaderIdempotencyKey = "idempotency-key"
	// HeaderDeadline is the client's deadline in Unix milliseconds.
	HeaderDeadline = "x-deadline"
)

// Call is one delivered request with the metadata its broker message carried.
type Call struct {
	Method         string
	Payload        []byte
	RequestID      string
	IdempotencyKey string
	// Deadline is the x-deadline header as sent, "" when absent.
	Deadline string
}

// Answer is a reply ready to send.
type Answer struct {
	Body      []byte
	RequestID string
	// Error is the error member of the reply, nil on success. Brokers that report errors in
	// their own headers read it.
	Error *Error
}

// Dispatcher runs calls through the router with the rules of plan section 11.2: a request ID for
// every call, a deadline that is the earlier of the client's and MaxTimeout, a call already past
// its deadline answered without running, recovered panics, and one access log line per call.
type Dispatcher struct {
	router     *Router
	maxTimeout time.Duration
	logger     *slog.Logger
	now        func() time.Time
}

// NewDispatcher builds a dispatcher. maxTimeout bounds every call (RPC_TIMEOUT_MAX).
func NewDispatcher(router *Router, maxTimeout time.Duration, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{router: router, maxTimeout: maxTimeout, logger: logger, now: time.Now}
}

// Answer runs one call. ctx should not be cancelled by a server shutdown: a call in flight
// finishes during the drain. The reply always encodes.
func (d *Dispatcher) Answer(ctx context.Context, c Call) Answer {
	start := d.now()
	id := c.RequestID
	if !requestid.Valid(id) {
		id = requestid.New()
	}
	ctx = requestid.With(ctx, id)
	ctx = logging.With(ctx, d.logger.With(slog.String("request_id", id)))

	data, err := d.run(ctx, c, start)
	reply := Reply{OK: err == nil, Data: data}
	if err != nil {
		reply.Data = nil
		reply.Error = ErrorOf(problem.From(err), id)
		logFailure(ctx, reply.Error, err)
	}
	body, encErr := json.Marshal(reply)
	if encErr != nil {
		// A handler returned something JSON cannot hold: a bug, answered as one.
		logging.From(ctx).ErrorContext(ctx, "encode reply", slog.Any("error", encErr))
		reply = Reply{Error: ErrorOf(problem.Blank(http.StatusInternalServerError), id)}
		body, _ = json.Marshal(reply)
	}
	code := "OK"
	if reply.Error != nil {
		code = reply.Error.Code
	}
	logging.From(ctx).InfoContext(ctx, "request",
		slog.String("method", c.Method), slog.String("code", code), slog.Duration("duration", d.now().Sub(start)))
	return Answer{Body: body, RequestID: id, Error: reply.Error}
}

func (d *Dispatcher) run(ctx context.Context, c Call, start time.Time) (data any, err error) {
	deadline := start.Add(d.maxTimeout)
	if c.Deadline != "" {
		ms, parseErr := strconv.ParseInt(c.Deadline, 10, 64)
		if parseErr != nil {
			return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", HeaderDeadline+" must be Unix milliseconds")
		}
		if client := time.UnixMilli(ms); client.Before(deadline) {
			deadline = client
		}
	}
	if !start.Before(deadline) {
		return nil, ledger.NewProblem(ledger.ErrUnavailable, "deadline-exceeded", "the deadline passed before the request ran; it was not executed")
	}
	h, ok := d.router.lookup(c.Method)
	if !ok {
		return nil, ledger.NewProblem(ledger.ErrNotFound, "unknown-method", fmt.Sprintf("no method %q", c.Method))
	}
	req, err := decode(c.Method, c.Payload, c.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	defer func() {
		if rec := recover(); rec != nil {
			logging.From(ctx).ErrorContext(ctx, "panic", slog.Any("panic", rec), slog.String("stack", string(debug.Stack())))
			data, err = nil, fmt.Errorf("panic: %v", rec)
		}
	}()
	return h(ctx, req)
}

// logFailure logs the cause of a failed call: at error level when the fault is ours, at info
// level when the request was refused. The cause's text goes to the log only.
func logFailure(ctx context.Context, e *Error, err error) {
	logger := logging.From(ctx)
	if e.Status >= 500 {
		logger.ErrorContext(ctx, "request failed", slog.String("code", e.Code), slog.Any("error", err))
		return
	}
	logger.InfoContext(ctx, "request rejected", slog.String("code", e.Code), slog.Any("error", err))
}

// DeadlineHeader renders a client deadline for HeaderDeadline.
func DeadlineHeader(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}
