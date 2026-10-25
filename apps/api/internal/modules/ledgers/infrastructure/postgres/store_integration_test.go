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

func startStore(t *testing.T) *Store {
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
	return NewStore(db)
}

func TestStoreIntegration(t *testing.T) {
	store := startStore(t)
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
	if _, err := store.CreateLedger(ctx, want); !errors.Is(err, ledger.ErrAlreadyExists) {
		t.Fatalf("duplicate CreateLedger error = %v, want ErrAlreadyExists", err)
	}
	if _, err := store.GetLedger(ctx, "missing"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("GetLedger(missing) error = %v, want ErrNotFound", err)
	}
	if _, err := store.CreateLedger(ctx, ledger.Ledger{Name: "b", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListLedgers(ctx, "a", 10)
	if err != nil || len(page) != 2 || page[0].Name != "b" || page[1].Name != "main" {
		t.Fatalf("ListLedgers = %+v, %v", page, err)
	}
	if err := store.ReplaceLedgerMetadata(ctx, "missing", nil); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("ReplaceLedgerMetadata(missing) error = %v", err)
	}

	rule := ledger.Rule{Pattern: "users:", Limit: ledger.LimitDebitsMustNotExceedCredits, History: true, Code: 7, CreatedAt: now, UpdatedAt: now}
	if _, err := store.PutRule(ctx, "main", rule); err != nil {
		t.Fatalf("PutRule: %v", err)
	}
	rules, err := store.ListRules(ctx, "main")
	if err != nil || len(rules) != 1 || rules[0].Code != 7 || !rules[0].History {
		t.Fatalf("ListRules = %+v, %v", rules, err)
	}
	if err := store.DeleteRule(ctx, "main", "users:"); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	if err := store.DeleteRule(ctx, "main", "users:"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("second DeleteRule error = %v", err)
	}

	first, err := store.CreateEventType(ctx, "main", "card.authorized", 0)
	if err != nil || first.Code != 2 {
		t.Fatalf("CreateEventType = %+v, %v", first, err)
	}
	if _, err := store.CreateEventType(ctx, "main", "card.settled", 2); !errors.Is(err, ledger.ErrAlreadyExists) {
		t.Fatalf("taken code error = %v", err)
	}
	if got, err := store.GetEventType(ctx, "main", "card.authorized"); err != nil || got.Code != 2 {
		t.Fatalf("GetEventType = %+v, %v", got, err)
	}
	if types, err := store.ListEventTypes(ctx, "main"); err != nil || len(types) != 1 {
		t.Fatalf("ListEventTypes = %+v, %v", types, err)
	}
}
