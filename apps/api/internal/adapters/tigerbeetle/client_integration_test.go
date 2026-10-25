//go:build integration

package tigerbeetle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func TestClientIntegration(t *testing.T) {
	address := devstack.StartTigerBeetle(t)

	client, err := New(0, []string{address})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(client.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}

	// Concurrent probes all succeed (they share one in-flight request; see Client.Ready).
	errs := make(chan error, 5)
	for range 5 {
		go func() { errs <- client.Ready(ctx) }()
	}
	for range 5 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent Ready: %v", err)
		}
	}

	id := client.NewID()
	if id.IsZero() {
		t.Fatal("NewID returned zero")
	}
}

func TestClientUnreachable(t *testing.T) {
	client, err := New(0, []string{"127.0.0.1:1"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(client.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err = client.Ready(ctx)
	if !errors.Is(err, ledger.ErrUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Ready error = %v, want ErrUnavailable wrapping DeadlineExceeded", err)
	}
}
