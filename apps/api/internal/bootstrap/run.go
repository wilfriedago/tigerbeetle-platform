package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// Runner is a server or a background worker. Run blocks until ctx is cancelled, then drains within
// its own shutdown timeout, and returns nil on a clean stop.
type Runner interface {
	Name() string
	Run(ctx context.Context) error
}

// Run starts every runner and returns when ctx is cancelled or one runner fails; the others are
// then cancelled too, because errgroup cancels its context on the first error. The resources close
// afterwards, in reverse order of construction.
func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, r := range a.runners {
		g.Go(func() error {
			if err := r.Run(gctx); err != nil {
				return fmt.Errorf("%s: %w", r.Name(), err)
			}
			return nil
		})
	}
	err := g.Wait()
	a.close()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
