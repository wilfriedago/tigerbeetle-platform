// Package cdc is the bookkeeping module's interface to TigerBeetle change data capture (ADR-0022):
// it reads the events the CDC job publishes to RabbitMQ and hands them to the ledger event relay.
package cdc

import (
	"context"
	"crypto/tls"
	"log/slog"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/amqp"
)

// Exchange is the fanout exchange the CDC job publishes to; deploy/compose declares it too.
const Exchange = "tbp.cdc"

// Relayer publishes change events; the application's LedgerEventRelay is one.
type Relayer interface {
	Relay(ctx context.Context, events ...domain.ChangeEvent) error
}

// Options configures a Consumer.
type Options struct {
	URL string
	// Queue is the relay queue, tbp.ledger-events.relay by default (CDC_QUEUE). It has a single
	// active consumer, so every replica may run the relay while one at a time reads, in order.
	Queue  string
	Relay  Relayer
	Logger *slog.Logger
	// TLS configures an amqps:// connection; nil trusts the system's CAs.
	TLS *tls.Config
}

// Consumer relays the queue one event at a time, in TigerBeetle timestamp order, and acknowledges
// an event only once the relay returned: Kafka has it. A failed relay is retried in place with
// capped backoff, which keeps the order; RabbitMQ keeps the event meanwhile.
type Consumer struct {
	opts       Options
	supervisor *amqp.Supervisor
	minBackoff time.Duration
	maxBackoff time.Duration
}

// NewConsumer builds a consumer. Nothing connects before Run.
func NewConsumer(opts Options) *Consumer {
	return &Consumer{
		opts:       opts,
		supervisor: amqp.New(opts.URL, "tbp-api cdc relay", opts.Logger, opts.TLS),
		minBackoff: 200 * time.Millisecond,
		maxBackoff: 10 * time.Second,
	}
}

// Run consumes until ctx ends, through broker restarts.
func (c *Consumer) Run(ctx context.Context) error {
	return c.supervisor.Run(ctx, c.session)
}

// Ready reports whether the connection is open and the relay queue answers a passive declare.
func (c *Consumer) Ready(ctx context.Context) error {
	return c.supervisor.Check(ctx, func(ch *amqp091.Channel) error {
		_, err := ch.QueueDeclarePassive(c.opts.Queue, true, false, false, false, singleActive)
		return err
	})
}

var singleActive = amqp091.Table{"x-single-active-consumer": true}

// Declare makes the exchange, the relay queue and their binding, as the compose definitions do;
// declarations are idempotent, so a broker without the definitions works too.
func Declare(ch *amqp091.Channel, queue string) error {
	if err := ch.ExchangeDeclare(Exchange, amqp091.ExchangeFanout, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, singleActive); err != nil {
		return err
	}
	return ch.QueueBind(queue, "", Exchange, false, nil)
}

func (c *Consumer) session(ctx context.Context, conn *amqp091.Connection) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()
	if err := Declare(ch, c.opts.Queue); err != nil {
		return err
	}
	// Events are relayed one at a time; the prefetch only saves round trips.
	if err := ch.Qos(64, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, c.opts.Queue, "tbp-api-relay", false, false, false, false, nil)
	if err != nil {
		return err
	}
	c.opts.Logger.InfoContext(ctx, "cdc relay consuming", slog.String("queue", c.opts.Queue))
	for d := range deliveries {
		if !c.handle(ctx, d) {
			// ctx ended during a retry: the delivery stays unacknowledged and RabbitMQ gives it to
			// the next consumer.
			return nil
		}
	}
	return nil
}

// handle relays one delivery and acknowledges it. It returns false when ctx ended first.
func (c *Consumer) handle(ctx context.Context, d amqp091.Delivery) bool {
	ev, err := Decode(d.Body)
	if err != nil {
		// A message that does not decode never will: dropped, with its body in the log.
		c.opts.Logger.ErrorContext(ctx, "cdc message dropped", slog.Any("error", err), slog.String("body", string(d.Body)))
		_ = d.Nack(false, false)
		return true
	}
	backoff := c.minBackoff
	for {
		err := c.opts.Relay.Relay(ctx, ev)
		if err == nil {
			if ackErr := d.Ack(false); ackErr != nil {
				// The event reaches Kafka again when RabbitMQ redelivers it; consumers deduplicate
				// on tb-timestamp.
				c.opts.Logger.WarnContext(ctx, "cdc ack failed", slog.Uint64("tb_timestamp", ev.Timestamp), slog.Any("error", ackErr))
			}
			return true
		}
		c.opts.Logger.WarnContext(ctx, "cdc relay failed, retrying", slog.Uint64("tb_timestamp", ev.Timestamp),
			slog.Duration("in", backoff), slog.Any("error", err))
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, c.maxBackoff)
	}
}
