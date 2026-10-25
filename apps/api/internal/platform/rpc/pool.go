package rpc

import (
	"context"
	"sync"
)

// Pool bounds how many requests a server runs at once (RPC_CONCURRENCY). A broker keeps the rest:
// RabbitMQ holds unacknowledged messages beyond the prefetch, and NATS buffers on the
// subscription.
type Pool struct {
	slots chan struct{}
	wg    sync.WaitGroup
}

// NewPool returns a pool of size slots, at least one.
func NewPool(size int) *Pool {
	return &Pool{slots: make(chan struct{}, max(size, 1))}
}

// Go runs fn on a goroutine once a slot is free. It returns ctx's error without running fn when
// ctx ends first.
func (p *Pool) Go(ctx context.Context, fn func()) error {
	select {
	case p.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	p.wg.Add(1)
	go func() {
		defer func() {
			<-p.slots
			p.wg.Done()
		}()
		fn()
	}()
	return nil
}

// Wait blocks until every running fn has returned, or ctx ends.
func (p *Pool) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
