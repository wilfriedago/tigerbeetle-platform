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
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

func startPostgres(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	db, err := platformpg.Open(ctx, devstack.StartPostgres(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(ctx, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return New(db)
}

func TestStoreIntegration(t *testing.T) {
	store := startPostgres(t)
	ctx := context.Background()

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
}
