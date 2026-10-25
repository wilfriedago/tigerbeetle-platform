//go:build integration

package postgres

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

func startPostgres(t *testing.T) *Store {
	t.Helper()
	store, err := New(context.Background(), devstack.StartPostgres(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

func TestStoreIntegration(t *testing.T) {
	store := startPostgres(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := store.Ready(ctx); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	if err := store.Migrate(ctx, logger); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Migrations are idempotent: a second run applies nothing and succeeds.
	if err := store.Migrate(ctx, logger); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	now := time.Date(2026, 10, 24, 10, 30, 0, 0, time.UTC)
	want := ledger.Ledger{Name: "main", Metadata: map[string]string{"env": "test"}, CreatedAt: now}
	created, err := store.CreateLedger(ctx, want)
	if err != nil {
		t.Fatalf("CreateLedger: %v", err)
	}
	if diff := cmp.Diff(want, created); diff != "" {
		t.Fatalf("CreateLedger mismatch (-want +got):\n%s", diff)
	}

	got, err := store.GetLedger(ctx, "main")
	if err != nil {
		t.Fatalf("GetLedger: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("GetLedger mismatch (-want +got):\n%s", diff)
	}

	if _, err := store.CreateLedger(ctx, want); !errors.Is(err, ledger.ErrAlreadyExists) {
		t.Fatalf("duplicate CreateLedger error = %v, want ErrAlreadyExists", err)
	}
	if _, err := store.GetLedger(ctx, "missing"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("GetLedger(missing) error = %v, want ErrNotFound", err)
	}
	if _, err := store.CreateLedger(ctx, ledger.Ledger{Name: "Not Valid", CreatedAt: now}); err == nil {
		t.Fatal("expected the CHECK constraint to reject an invalid name")
	}

	// Readiness fails fast once the deadline has passed.
	expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer cancel()
	if err := store.Ready(expired); !errors.Is(err, ledger.ErrUnavailable) {
		t.Fatalf("Ready with expired context error = %v, want ErrUnavailable", err)
	}
}
