package amqprpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// directReplyTo is RabbitMQ's pseudo-queue for replies: no queue to declare, and the reply comes
// back on the channel that sent the request (https://www.rabbitmq.com/docs/direct-reply-to).
const directReplyTo = "amq.rabbitmq.reply-to"

// Client calls the RPC methods over RabbitMQ. The tests and the end-to-end suite use it; it shows
// what any client does.
type Client struct {
	conn  *amqp091.Connection
	ch    *amqp091.Channel
	queue string

	mu      sync.Mutex
	pending map[string]chan amqp091.Delivery
}

// Dial connects and starts listening for replies.
func Dial(url, queue string) (*Client, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("amqprpc: dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("amqprpc: channel: %w", err)
	}
	// Direct reply-to needs automatic acknowledgement.
	replies, err := ch.Consume(directReplyTo, "", true, false, false, false, nil)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("amqprpc: consume replies: %w", err)
	}
	c := &Client{conn: conn, ch: ch, queue: queue, pending: map[string]chan amqp091.Delivery{}}
	go c.route(replies)
	return c, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) route(replies <-chan amqp091.Delivery) {
	for d := range replies {
		c.mu.Lock()
		waiter, ok := c.pending[d.CorrelationId]
		delete(c.pending, d.CorrelationId)
		c.mu.Unlock()
		if ok {
			waiter <- d
		}
	}
}

// Call sends one request and waits for its reply until ctx ends. The deadline of ctx travels as
// x-deadline and as the message expiration, so a request nobody took in time is dropped. It
// returns the reply body and the request ID the server used.
func (c *Client) Call(ctx context.Context, method string, env rpc.Envelope, h rpc.Headers) (rpc.Answer, error) {
	body, err := json.Marshal(env)
	if err != nil {
		return rpc.Answer{}, err
	}
	correlation := requestid.New()
	waiter := make(chan amqp091.Delivery, 1)
	c.mu.Lock()
	c.pending[correlation] = waiter
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, correlation)
		c.mu.Unlock()
	}()

	headers := amqp091.Table{}
	for name, value := range map[string]string{
		rpc.HeaderRequestID: h.RequestID, rpc.HeaderIdempotencyKey: h.IdempotencyKey,
		rpc.HeaderAuthorization: h.Authorization, rpc.HeaderOrganization: h.Organization,
	} {
		if value != "" {
			headers[name] = value
		}
	}
	msg := amqp091.Publishing{
		ContentType: "application/json", Type: method, CorrelationId: correlation, ReplyTo: directReplyTo,
		Headers: headers, Body: body,
	}
	if dl, ok := ctx.Deadline(); ok {
		headers[rpc.HeaderDeadline] = rpc.DeadlineHeader(dl)
		msg.Expiration = strconv.FormatInt(max(time.Until(dl).Milliseconds(), 1), 10)
	}
	if err := c.ch.PublishWithContext(ctx, Exchange, c.queue, false, false, msg); err != nil {
		return rpc.Answer{}, fmt.Errorf("amqprpc: publish: %w", err)
	}
	select {
	case d := <-waiter:
		return rpc.Answer{Body: d.Body, RequestID: header(d.Headers, rpc.HeaderRequestID)}, nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return rpc.Answer{}, rpc.ErrTimeout
		}
		return rpc.Answer{}, ctx.Err()
	}
}
