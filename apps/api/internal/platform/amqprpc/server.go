// Package amqprpc serves the broker RPC methods over RabbitMQ (ADR-0021, plan section 11.2). Every
// replica consumes one durable queue, so each request runs once; the prefetch is the worker pool
// size, so a replica holds no more requests than it runs. A request names its method in the
// message type, its reply queue in reply_to (normally RabbitMQ's direct reply-to) and its
// correlation ID; the reply carries the same correlation ID, and the request is acknowledged only
// after the reply is published.
package amqprpc

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/amqp"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// Exchange is the durable direct exchange the request queues are bound to.
const Exchange = "tbp.rpc"

// Options configures a Server.
type Options struct {
	URL string
	// Queue is the durable request queue, tbp.rpc.v1 by default (AMQP_RPC_QUEUE). It is bound to
	// Exchange with its own name as routing key.
	Queue           string
	Concurrency     int
	ShutdownTimeout time.Duration
	Dispatcher      *rpc.Dispatcher
	Logger          *slog.Logger
	// TLS configures an amqps:// connection; nil trusts the system's CAs.
	TLS *tls.Config
}

// Server consumes the request queue and answers on the reply queues.
type Server struct {
	opts       Options
	supervisor *amqp.Supervisor
	pool       *rpc.Pool
}

// New builds a server. Nothing connects before Run.
func New(opts Options) *Server {
	return &Server{
		opts:       opts,
		supervisor: amqp.New(opts.URL, "tbp-api rpc", opts.Logger, opts.TLS),
		pool:       rpc.NewPool(opts.Concurrency),
	}
}

// Run serves until ctx ends, through broker restarts, then lets the requests in flight finish
// within ShutdownTimeout. RabbitMQ requeues whatever was not acknowledged.
func (s *Server) Run(ctx context.Context) error {
	err := s.supervisor.Run(ctx, s.session)
	drain, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.opts.ShutdownTimeout)
	defer cancel()
	if waitErr := s.pool.Wait(drain); waitErr != nil {
		s.opts.Logger.WarnContext(ctx, "amqp rpc drain timed out; RabbitMQ requeues the unacknowledged requests")
	}
	return err
}

// Ready reports whether the connection is open and the request queue answers a passive declare.
func (s *Server) Ready(ctx context.Context) error {
	return s.supervisor.Check(ctx, func(ch *amqp091.Channel) error {
		_, err := ch.QueueDeclarePassive(s.opts.Queue, true, false, false, false, nil)
		return err
	})
}

// session declares the topology and consumes until ctx ends or the connection drops.
func (s *Server) session(ctx context.Context, conn *amqp091.Connection) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()
	if err := Declare(ch, s.opts.Queue); err != nil {
		return err
	}
	if err := ch.Qos(s.opts.Concurrency, 0, false); err != nil {
		return err
	}
	const tag = "tbp-api"
	deliveries, err := ch.ConsumeWithContext(ctx, s.opts.Queue, tag, false, false, false, false, nil)
	if err != nil {
		return err
	}
	s.opts.Logger.InfoContext(ctx, "amqp rpc consuming", slog.String("queue", s.opts.Queue), slog.Int("concurrency", s.opts.Concurrency))

	// Requests already delivered are answered even when ctx ends; the pool's drain bounds that.
	work := context.WithoutCancel(ctx)
	for d := range deliveries {
		if err := s.pool.Go(work, func() { s.handle(work, ch, d) }); err != nil {
			return err
		}
	}
	// The channel closes when ctx ends (the consumer is cancelled) or the connection drops. Wait
	// for the requests in flight on this channel, so their acknowledgements are not lost to a
	// channel closed under them.
	drain, cancel := context.WithTimeout(work, s.opts.ShutdownTimeout)
	defer cancel()
	_ = s.pool.Wait(drain)
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("amqp rpc: deliveries closed")
}

// Declare makes the exchange, the durable request queue and their binding. Clients may call it
// too; declarations are idempotent.
func Declare(ch *amqp091.Channel, queue string) error {
	if err := ch.ExchangeDeclare(Exchange, amqp091.ExchangeDirect, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}
	return ch.QueueBind(queue, queue, Exchange, false, nil)
}

// handle answers one request and acknowledges it. A malformed request is answered with an error
// and acknowledged too: requeueing it would only fail again.
func (s *Server) handle(ctx context.Context, ch *amqp091.Channel, d amqp091.Delivery) {
	answer := s.opts.Dispatcher.Answer(ctx, rpc.Call{
		Method:         d.Type,
		Payload:        d.Body,
		RequestID:      header(d.Headers, rpc.HeaderRequestID),
		IdempotencyKey: header(d.Headers, rpc.HeaderIdempotencyKey),
		Deadline:       header(d.Headers, rpc.HeaderDeadline),
		Authorization:  header(d.Headers, rpc.HeaderAuthorization),
		Organization:   header(d.Headers, rpc.HeaderOrganization),
	})
	if d.ReplyTo != "" {
		publish, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := ch.PublishWithContext(publish, "", d.ReplyTo, false, false, amqp091.Publishing{
			ContentType:   "application/json",
			CorrelationId: d.CorrelationId,
			Type:          d.Type,
			Headers:       amqp091.Table{rpc.HeaderRequestID: answer.RequestID},
			Body:          answer.Body,
		})
		cancel()
		if err != nil {
			s.opts.Logger.WarnContext(ctx, "amqp rpc reply not published", slog.String("request_id", answer.RequestID), slog.Any("error", err))
		}
	}
	if err := d.Ack(false); err != nil {
		s.opts.Logger.WarnContext(ctx, "amqp rpc ack failed; RabbitMQ will redeliver", slog.String("request_id", answer.RequestID), slog.Any("error", err))
	}
}

// header reads a string header; AMQP tables may hold it as a string or as bytes.
func header(t amqp091.Table, key string) string {
	switch v := t[key].(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}
