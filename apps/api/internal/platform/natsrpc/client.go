package natsrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// Client calls the RPC methods over NATS. The tests and the end-to-end suite use it; it shows
// what any client does, including the two failures evrone's client confused: no reply in time,
// and nobody listening.
type Client struct {
	nc *nats.Conn
}

// Dial connects to NATS.
func Dial(url string) (*Client, error) {
	nc, err := nats.Connect(url, nats.Name("tbp-api rpc client"))
	if err != nil {
		return nil, fmt.Errorf("natsrpc: connect: %w", err)
	}
	return &Client{nc: nc}, nil
}

// Close closes the connection.
func (c *Client) Close() { c.nc.Close() }

// Call sends one request and waits for its reply until ctx ends; the deadline of ctx travels as
// X-Deadline. A timeout returns rpc.ErrTimeout, a subject nobody serves rpc.ErrNoResponders.
func (c *Client) Call(ctx context.Context, method string, env rpc.Envelope, key, requestID string) (rpc.Answer, error) {
	body, err := json.Marshal(env)
	if err != nil {
		return rpc.Answer{}, err
	}
	msg := nats.NewMsg(SubjectPrefix + "." + strings.TrimPrefix(method, "v1."))
	msg.Data = body
	if requestID != "" {
		msg.Header.Set(HeaderRequestID, requestID)
	}
	if key != "" {
		msg.Header.Set(HeaderIdempotencyKey, key)
	}
	if dl, ok := ctx.Deadline(); ok {
		msg.Header.Set(HeaderDeadline, rpc.DeadlineHeader(dl))
	}
	reply, err := c.nc.RequestMsgWithContext(ctx, msg)
	switch {
	case errors.Is(err, nats.ErrNoResponders):
		return rpc.Answer{}, rpc.ErrNoResponders
	case errors.Is(err, nats.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return rpc.Answer{}, rpc.ErrTimeout
	case err != nil:
		return rpc.Answer{}, fmt.Errorf("natsrpc: request: %w", err)
	}
	return rpc.Answer{Body: reply.Data, RequestID: reply.Header.Get(HeaderRequestID)}, nil
}
