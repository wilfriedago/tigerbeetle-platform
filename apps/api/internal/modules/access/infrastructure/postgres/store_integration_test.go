//go:build integration

package postgres

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

func startStore(t *testing.T) (*Store, *platformpg.DB) {
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
	return NewStore(db), db
}

var staff = ledger.Principal{Issuer: "kc", Subject: "sam", Name: "Sam", Kind: ledger.KindPerson, Staff: true}

func TestOrganizationsIntegration(t *testing.T) {
	store, _ := startStore(t)
	ctx := context.Background()

	// A creation interrupted before Keycloak answered left a row without its ID; the retry
	// completes it and keeps the first row.
	first, err := store.SaveOrganization(ctx, domain.NewOrganization{Alias: "initech", Name: "Initech", CreatedBy: staff})
	if err != nil {
		t.Fatal(err)
	}
	if first.KeycloakID != "" || first.Status != domain.OrganizationActive || first.ID.IsZero() {
		t.Fatalf("first save = %+v", first)
	}
	kcID := "6f1d4a8e-2f1c-4b5e-9a57-3c2d1e0f9a8b"
	again, err := store.SaveOrganization(ctx, domain.NewOrganization{Alias: "initech", Name: "Other", KeycloakID: kcID, CreatedBy: staff})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID || again.Name != "Initech" || again.KeycloakID != kcID {
		t.Fatalf("retried save = %+v, want the first row with the Keycloak ID", again)
	}
	if _, err := store.SaveOrganization(ctx, domain.NewOrganization{Alias: "bad", Name: "Bad", KeycloakID: "not-a-uuid"}); err == nil || errors.Is(err, ledger.ErrInvalid) {
		t.Fatalf("a malformed Keycloak ID = %v, want a server error", err)
	}

	if err := store.SetKeycloakID(ctx, "default", kcID[:35]+"c"); err != nil {
		t.Fatal(err)
	}
	renamed, err := store.RenameOrganization(ctx, "initech", "Initech Corp")
	if err != nil || renamed.Name != "Initech Corp" {
		t.Fatalf("RenameOrganization = %+v, %v", renamed, err)
	}
	suspended, err := store.SetOrganizationStatus(ctx, "initech", domain.OrganizationSuspended, "unpaid")
	if err != nil || suspended.Status != domain.OrganizationSuspended || suspended.SuspensionReason != "unpaid" {
		t.Fatalf("SetOrganizationStatus = %+v, %v", suspended, err)
	}
	if first.CreatedByName != "Sam" {
		t.Errorf("created by %q, want Sam", first.CreatedByName)
	}
	active, err := store.SetOrganizationStatus(ctx, "initech", domain.OrganizationActive, "ignored")
	if err != nil || active.SuspensionReason != "" {
		t.Fatalf("reactivate = %+v, %v; want no reason", active, err)
	}
	if _, err := store.RenameOrganization(ctx, "nope", "Nope"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("RenameOrganization(nope) = %v, want ErrNotFound", err)
	}

	page, err := store.ListOrganizations(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	aliases := make([]string, len(page))
	for i, o := range page {
		aliases[i] = o.Alias
	}
	if diff := cmp.Diff([]string{"default", "initech"}, aliases); diff != "" {
		t.Errorf("ListOrganizations (-want +got):\n%s", diff)
	}
	if page[0].KeycloakID == "" {
		t.Error("SetKeycloakID did not record the default organization's ID")
	}
	rest, err := store.ListOrganizations(ctx, "default", 10)
	if err != nil || len(rest) != 1 || rest[0].Alias != "initech" {
		t.Errorf("ListOrganizations after default = %+v, %v", rest, err)
	}
}

func TestGrantsIntegration(t *testing.T) {
	store, db := startStore(t)
	ctx := context.Background()
	if _, err := db.Pool().Exec(ctx, `
		INSERT INTO organizations (id, alias, name) VALUES ('00000000-0000-0000-0000-000000000002', 'globex', 'Globex');
		INSERT INTO ledgers (organization_id, name) VALUES ('00000000-0000-0000-0000-000000000001', 'payments');`); err != nil {
		t.Fatal(err)
	}
	acme := ledger.WithTenant(ctx, ledger.DefaultTenant)
	globex := ledger.WithTenant(ctx, ledger.Tenant{ID: ledger.ID{15: 2}, Alias: "globex"})
	ada := domain.Holder{Issuer: "kc", Subject: "ada"}
	bob := domain.Holder{Issuer: "kc", Subject: "bob"}

	put := func(ctx context.Context, h domain.Holder, ledgerName string, role ledger.Role) {
		t.Helper()
		if err := store.PutGrant(ctx, domain.HeldGrant{Holder: h, Grant: domain.Grant{Ledger: ledgerName, Role: role}}, staff); err != nil {
			t.Fatalf("PutGrant(%s, %q, %s): %v", h.Subject, ledgerName, role, err)
		}
	}
	put(acme, ada, "", ledger.RoleOwner)
	put(acme, bob, "", ledger.RoleOwner)
	put(acme, bob, "", ledger.RoleMember) // replaces the owner role
	put(acme, bob, "payments", ledger.RoleViewer)
	put(acme, bob, "payments", ledger.RoleOperator) // replaces viewer
	put(globex, bob, "", ledger.RoleOwner)

	// payments exists only in the default organization.
	err := store.PutGrant(globex, domain.HeldGrant{Holder: bob, Grant: domain.Grant{Ledger: "payments", Role: ledger.RoleViewer}}, staff)
	if !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("a grant on another organization's ledger = %v, want ErrNotFound", err)
	}

	got, err := store.GrantsIn(acme)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.HeldGrant{
		{Holder: ada, Grant: domain.Grant{Role: ledger.RoleOwner}, GrantedByName: "Sam"},
		{Holder: bob, Grant: domain.Grant{Role: ledger.RoleMember}, GrantedByName: "Sam"},
		{Holder: bob, Grant: domain.Grant{Ledger: "payments", Role: ledger.RoleOperator}, GrantedByName: "Sam"},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty(), cmpopts.IgnoreFields(domain.HeldGrant{}, "GrantedAt")); diff != "" {
		t.Errorf("GrantsIn (-want +got):\n%s", diff)
	}
	for _, g := range got {
		if g.GrantedAt.IsZero() || g.GrantedByName != "Sam" {
			t.Errorf("grant %+v: want when and by whom", g)
		}
	}
	owners, err := store.OwnersIn(ctx, []ledger.ID{ledger.DefaultOrganizationID, {15: 2}})
	if err != nil || len(owners[ledger.DefaultOrganizationID]) != 1 || len(owners[ledger.ID{15: 2}]) != 1 {
		t.Errorf("OwnersIn = %+v, %v; want ada in default, bob in globex", owners, err)
	}
	across, err := store.GrantsAcross(ctx, "kc", "bob")
	if err != nil || len(across) != 3 || across[0].Alias != "default" || across[len(across)-1].Alias != "globex" {
		t.Errorf("GrantsAcross(bob) = %+v, %v", across, err)
	}
	if n, err := store.CountOwners(acme); err != nil || n != 1 {
		t.Errorf("CountOwners = %d, %v; want 1", n, err)
	}

	if err := store.RevokeGrant(acme, bob, "payments"); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeGrant(acme, bob, "payments"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("revoking twice = %v, want ErrNotFound", err)
	}
	if err := store.RevokeGrants(acme, bob); err != nil {
		t.Fatal(err)
	}
	if grants, _ := store.GrantsOf(acme, "kc", "bob"); len(grants) != 0 {
		t.Errorf("bob still holds %v in acme", grants)
	}
	if grants, _ := store.GrantsOf(globex, "kc", "bob"); len(grants) != 1 {
		t.Errorf("revoking in acme touched globex: %v", grants)
	}
	if _, err := store.GrantsIn(ctx); err == nil {
		t.Error("GrantsIn without a tenant succeeded")
	}
}
