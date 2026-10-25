//go:build integration

package postgres

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

func TestFootprintsIntegration(t *testing.T) {
	ctx := context.Background()
	db, err := platformpg.Open(ctx, devstack.StartPostgres(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(ctx, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// The ledgers module owns the ledgers table; this test inserts its rows directly.
	if _, err := db.Pool().Exec(ctx, "INSERT INTO ledgers (name) VALUES ('eu'), ('empty')"); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)

	eur, _ := ledger.ParseAsset("EUR/2")
	usd, _ := ledger.ParseAsset("USD/2")
	assets, err := store.EnsureAssets(ctx, "eu", []ledger.Asset{usd, eur})
	if err != nil {
		t.Fatalf("EnsureAssets: %v", err)
	}
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	candidates := []domain.AccountRow{
		{ID: ledger.ID{15: 1}, Address: "users:1:wallet", Asset: "EUR/2", TBLedger: assets["EUR/2"].TBLedger, Code: 1, Limit: ledger.LimitNone, RulePattern: "*", CreatedAt: now},
		{ID: ledger.ID{15: 2}, Address: "users:1:wallet", Asset: "USD/2", TBLedger: assets["USD/2"].TBLedger, Code: 1, Limit: ledger.LimitNone, RulePattern: "*", CreatedAt: now},
		{ID: ledger.ID{15: 3}, Address: "world", Asset: "EUR/2", TBLedger: assets["EUR/2"].TBLedger, Code: 1, Limit: ledger.LimitNone, RulePattern: "world", CreatedAt: now},
	}
	if _, err := store.EnsureAccounts(ctx, "eu", candidates); err != nil {
		t.Fatalf("EnsureAccounts: %v", err)
	}

	got, err := store.LedgerFootprints(ctx, []string{"eu", "empty", "missing"})
	if err != nil {
		t.Fatalf("LedgerFootprints: %v", err)
	}
	want := map[string]ledger.Footprint{
		"eu":      {Assets: []string{"EUR/2", "USD/2"}, Accounts: 2},
		"empty":   {Assets: []string{}, Accounts: 0},
		"missing": {Assets: []string{}, Accounts: 0},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("LedgerFootprints mismatch (-want +got):\n%s", diff)
	}
	usage, err := store.EventTypeUsage(ctx, "eu")
	if err != nil || len(usage) != 0 {
		t.Fatalf("EventTypeUsage = %v, %v", usage, err)
	}
	row, err := store.GetAddress(ctx, "eu", "users:1:wallet")
	if err != nil || len(row.Accounts) != 2 {
		t.Fatalf("GetAddress = %+v, %v", row, err)
	}
}
