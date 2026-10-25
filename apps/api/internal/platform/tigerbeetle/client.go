// Package tigerbeetle owns the TigerBeetle client of the process: creating it from the cluster
// addresses, readiness, and bounded calls that respect the caller's context. The bookkeeping
// module builds its ledger store on top of it. The client uses cgo, so only the API binary and
// its tests import this package.
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
		return nil, fmt.Errorf("tigerbeetle: new client: %w", MapClientError(err))
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
			return fmt.Errorf("tigerbeetle: nop: %w", MapClientError(p.err))
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("tigerbeetle: nop: %w: %w", ledger.ErrUnavailable, ctx.Err())
	}
}

// MapClientError wraps client-level failures as ledger.ErrUnavailable, or ledger.ErrInvalid for an
// address the client refuses. Result codes of accounts and transfers are mapped by the store.
func MapClientError(err error) error {
	switch {
	case errors.Is(err, tb.ErrInvalidAddress), errors.Is(err, tb.ErrAddressLimitExceeded):
		return fmt.Errorf("%w: %w", ledger.ErrInvalid, err)
	default:
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	}
}

// Call runs a client request and waits for it or for the context. The client has no per-call
// cancellation and retries an unreachable cluster indefinitely, so on a deadline the request keeps
// running in its goroutine and may still land. Writes cope with that through the pending
// idempotency record and the reconciler (ADR-0007).
func Call[T any](ctx context.Context, c *Client, op string, fn func(client tb.Client) (T, error)) (T, error) {
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	type result struct {
		value T
		err   error
	}
	done := make(chan result, 1)
	go func() {
		v, err := fn(c.tb)
		done <- result{value: v, err: err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			var zero T
			return zero, fmt.Errorf("tigerbeetle: %s: %w", op, MapClientError(r.err))
		}
		return r.value, nil
	case <-ctx.Done():
		var zero T
		return zero, fmt.Errorf("tigerbeetle: %s: %w: %w", op, ledger.ErrUnavailable, ctx.Err())
	}
}
