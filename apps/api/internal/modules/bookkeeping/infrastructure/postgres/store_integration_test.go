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
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// otherTenant is a second organization; the tests give it a ledger with the same name as the
// default tenant's to prove that nothing crosses between them (ADR-0026).
var otherTenant = ledger.Tenant{ID: ledger.ID{15: 2}, Alias: "globex"}

func TestFootprintsIntegration(t *testing.T) {
	ctx := ledger.WithTenant(context.Background(), ledger.DefaultTenant)
	db, err := platformpg.Open(ctx, devstack.StartPostgres(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(ctx, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// The ledgers module owns the ledgers table; this test inserts its rows directly.
	if _, err := db.Pool().Exec(ctx, `
		INSERT INTO organizations (id, alias, name) VALUES ('00000000-0000-0000-0000-000000000002', 'globex', 'Globex');
		INSERT INTO ledgers (organization_id, name) VALUES
			('00000000-0000-0000-0000-000000000001', 'eu'),
			('00000000-0000-0000-0000-000000000001', 'empty'),
			('00000000-0000-0000-0000-000000000002', 'eu')`); err != nil {
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

	// The other organization's ledger "eu" shares the name and nothing else.
	other := ledger.WithTenant(context.Background(), otherTenant)
	if _, err := store.GetAddress(other, "eu", "users:1:wallet"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("GetAddress in the other tenant error = %v, want ErrNotFound", err)
	}
	otherPrints, err := store.LedgerFootprints(other, []string{"eu"})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]ledger.Footprint{"eu": {Assets: []string{}, Accounts: 0}}, otherPrints); diff != "" {
		t.Fatalf("LedgerFootprints in the other tenant (-want +got):\n%s", diff)
	}
	otherAssets, err := store.EnsureAssets(other, "eu", []ledger.Asset{eur})
	if err != nil {
		t.Fatalf("EnsureAssets in the other tenant: %v", err)
	}
	if otherAssets["EUR/2"].TBLedger == assets["EUR/2"].TBLedger {
		t.Fatal("two organizations share a TigerBeetle ledger number")
	}
	if _, err := store.EnsureAccounts(other, "eu", candidates[:1]); err == nil {
		t.Fatal("EnsureAccounts reused an account ID across organizations")
	}

	// A context without a tenant fails instead of reading every organization.
	if _, err := store.ListAssets(context.Background(), "eu"); err == nil {
		t.Fatal("ListAssets without a tenant succeeded")
	}
}
