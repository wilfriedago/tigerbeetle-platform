//go:build integration

package postgres

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func TestDBIntegration(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, devstack.StartPostgres(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := db.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	if err := db.Migrate(ctx, logger); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Migrations are idempotent: a second run applies nothing and succeeds.
	if err := db.Migrate(ctx, logger); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if v, err := db.SchemaVersion(ctx); err != nil || v == 0 {
		t.Fatalf("SchemaVersion = %d, %v", v, err)
	}
	if v, err := db.ServerVersion(ctx); err != nil || v == "" {
		t.Fatalf("ServerVersion = %q, %v", v, err)
	}

	// Readiness fails fast once the deadline has passed.
	expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer cancel()
	if err := db.Ready(expired); !errors.Is(err, ledger.ErrUnavailable) {
		t.Fatalf("Ready with expired context error = %v, want ErrUnavailable", err)
	}
}
