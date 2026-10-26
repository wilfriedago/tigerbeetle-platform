package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application/directorytest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

func staff() context.Context {
	return ledger.WithPrincipal(context.Background(), ledger.Principal{Issuer: "kc", Subject: "sam", Kind: ledger.KindPerson, Staff: true})
}

func newPlatform() (platformOrganizations, fakeOrgs, *directorytest.Directory, grantsBySubject) {
	dir := directorytest.New("kc")
	dir.AddPerson("bob", "bob@example.com")
	orgs := fakeOrgs{}
	grants := grantsBySubject{}
	authz := authorizer{grants: grants}
	m := &members{authz: authz, orgs: orgs, grants: grants, dir: dir, clock: ledger.ClockFunc(time.Now)}
	return platformOrganizations{authz: authz, orgs: orgs, grants: grants, members: m}, orgs, dir, grants
}

func TestCreateOrganization(t *testing.T) {
	u, orgs, dir, grants := newPlatform()
	in := CreateOrganization{Alias: "initech", Name: " Initech ", Owner: Invite{Email: "bob@example.com"}}
	v, err := u.Create(staff(), in)
	if err != nil {
		t.Fatal(err)
	}
	if id, _ := dir.OrganizationID(context.Background(), "initech"); v.Alias != "initech" || v.Name != "Initech" || orgs["initech"].KeycloakID != id {
		t.Errorf("Create = %+v, stored %+v", v, orgs["initech"])
	}
	if roleOf(grants, "bob", "") != ledger.RoleOwner || len(dir.Emails()) != 1 || dir.Emails()[0] != "invite bob" {
		t.Errorf("owner: grants %v, emails %v; want bob invited as owner", grants["bob"], dir.Emails())
	}
	if _, err := u.Create(staff(), in); !errors.Is(err, ledger.ErrAlreadyExists) {
		t.Errorf("Create twice = %v, want ErrAlreadyExists", err)
	}
}

func TestCreateOrganizationFinishesAnInterruptedOne(t *testing.T) {
	u, orgs, dir, grants := newPlatform()
	// A first attempt created the organization in Keycloak and stopped.
	id := dir.AddOrganization("initech")
	if _, err := u.Create(staff(), CreateOrganization{Alias: "initech", Name: "Initech", Owner: Invite{Email: "new@example.com"}}); err != nil {
		t.Fatal(err)
	}
	if orgs["initech"].KeycloakID != id || len(grants) != 1 {
		t.Errorf("stored %+v, grants %v", orgs["initech"], grants)
	}
}

func TestPlatformOrganizationsNeedStaff(t *testing.T) {
	u, orgs, _, _ := newPlatform()
	orgs["acme"] = domain.Organization{Alias: "acme", Name: "Acme", Status: domain.OrganizationActive}
	person := ledger.WithPrincipal(context.Background(), ledger.Principal{Issuer: "kc", Subject: "ada", Kind: ledger.KindPerson})
	calls := map[string]func(context.Context) error{
		"list":   func(ctx context.Context) error { _, _, err := u.List(ctx, "", 0); return err },
		"get":    func(ctx context.Context) error { _, err := u.Get(ctx, "acme"); return err },
		"rename": func(ctx context.Context) error { _, err := u.Rename(ctx, "acme", "Acme Corp"); return err },
		"status": func(ctx context.Context) error {
			_, err := u.SetStatus(ctx, "acme", domain.OrganizationSuspended)
			return err
		},
		"create": func(ctx context.Context) error {
			_, err := u.Create(ctx, CreateOrganization{Alias: "x", Name: "X", Owner: Invite{Email: "x@example.com"}})
			return err
		},
	}
	for name, call := range calls {
		if err := call(person); !errors.Is(err, ledger.ErrForbidden) {
			t.Errorf("%s by a person = %v, want ErrForbidden", name, err)
		}
	}
}

func TestOrganizationInput(t *testing.T) {
	u, orgs, _, _ := newPlatform()
	orgs["acme"] = domain.Organization{Alias: "acme", Name: "Acme", Status: domain.OrganizationActive, KeycloakID: "00000000-0000-4000-8000-00000000ac3e"}
	tests := []struct {
		name string
		err  error
	}{
		{"bad alias", func() error {
			_, err := u.Create(staff(), CreateOrganization{Alias: "Acme!", Name: "A", Owner: Invite{Email: "a@example.com"}})
			return err
		}()},
		{"empty name", func() error { _, err := u.Rename(staff(), "acme", "  "); return err }()},
		{"bad status", func() error { _, err := u.SetStatus(staff(), "acme", "closed"); return err }()},
		{"bad owner email", func() error {
			_, err := u.Create(staff(), CreateOrganization{Alias: "ok", Name: "Ok", Owner: Invite{Email: "nope"}})
			return err
		}()},
	}
	for _, tt := range tests {
		if !errors.Is(tt.err, ledger.ErrInvalid) {
			t.Errorf("%s = %v, want ErrInvalid", tt.name, tt.err)
		}
	}
	v, err := u.SetStatus(staff(), "acme", domain.OrganizationSuspended)
	if err != nil || v.Status != domain.OrganizationSuspended {
		t.Errorf("SetStatus = %+v, %v", v, err)
	}
	v, err = u.Rename(staff(), "acme", "Acme Corp")
	if err != nil || v.Name != "Acme Corp" {
		t.Errorf("Rename = %+v, %v", v, err)
	}
}

func TestListOrganizationsPages(t *testing.T) {
	u, orgs, _, _ := newPlatform()
	for _, a := range []string{"c", "a", "b"} {
		orgs[a] = domain.Organization{Alias: a, Name: a}
	}
	page, next, err := u.List(staff(), "", 2)
	if err != nil || len(page) != 2 || page[0].Alias != "a" || next == "" {
		t.Fatalf("first page = %v, %q, %v", page, next, err)
	}
	page, next, err = u.List(staff(), next, 2)
	if err != nil || len(page) != 1 || page[0].Alias != "c" || next != "" {
		t.Fatalf("second page = %v, %q, %v", page, next, err)
	}
}
