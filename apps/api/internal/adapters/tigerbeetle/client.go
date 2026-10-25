// Package tigerbeetle implements the ledger port with the official Go client. It owns the
// mapping between TigerBeetle types and the domain, including identifier byte order and, as the
// ledger use cases arrive, result codes to domain errors.
package tigerbeetle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	tb "github.com/tigerbeetle/tigerbeetle-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Client wraps a TigerBeetle client. One Client per process: the client batches requests from
// every goroutine and keeps a single connection per replica.
type Client struct {
	tb tb.Client
	// timeout bounds each request made for an API call; zero means only the caller's context.
	timeout time.Duration

	// The TigerBeetle client has no per-call cancellation and retries an unreachable cluster
	// indefinitely. Ready therefore runs at most one probe at a time and lets concurrent callers
	// share it, so a cluster outage cannot pile up goroutines behind the readiness endpoint.
	mu       sync.Mutex
	inflight *probe
}

// probe is one in-flight Nop round trip. done is closed once err is set, so any number of
// waiters can read the same outcome.
type probe struct {
	done chan struct{}
	err  error
}

// Option configures a Client.
type Option func(*Client)

// WithRequestTimeout bounds every request with the given duration, on top of the caller's context.
func WithRequestTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// New creates a client for the cluster. It does not fail when the cluster is unreachable; the
// connection is established in the background and Ready reports the state.
func New(clusterID uint64, addresses []string, opts ...Option) (*Client, error) {
	client, err := tb.NewClient(tb.ToUint128(clusterID), addresses)
	if err != nil {
		return nil, fmt.Errorf("tigerbeetle: new client: %w", mapClientError(err))
	}
	c := &Client{tb: client}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Close releases the client. Requests in flight fail with ErrClientClosed.
func (c *Client) Close() {
	c.tb.Close()
}

// Ready sends a no-op request to the cluster and waits for the reply or the context deadline.
func (c *Client) Ready(ctx context.Context) error {
	c.mu.Lock()
	p := c.inflight
	if p == nil {
		p = &probe{done: make(chan struct{})}
		c.inflight = p
		go func() {
			p.err = c.tb.Nop()
			c.mu.Lock()
			c.inflight = nil
			c.mu.Unlock()
			close(p.done)
		}()
	}
	c.mu.Unlock()

	select {
	case <-p.done:
		if p.err != nil {
			return fmt.Errorf("tigerbeetle: nop: %w", mapClientError(p.err))
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("tigerbeetle: nop: %w: %w", ledger.ErrUnavailable, ctx.Err())
	}
}

// NewID mints a time-ordered 128-bit identifier with the client's helper (ADR-0006).
func (c *Client) NewID() ledger.ID {
	return ToDomainID(tb.ID())
}

// mapClientError wraps client-level failures as ledger.ErrUnavailable. Result codes for accounts
// and transfers get their own mapping with the ledger use cases.
func mapClientError(err error) error {
	switch {
	case errors.Is(err, tb.ErrInvalidAddress), errors.Is(err, tb.ErrAddressLimitExceeded):
		return fmt.Errorf("%w: %w", ledger.ErrInvalid, err)
	default:
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	}
}
