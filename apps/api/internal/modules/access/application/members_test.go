package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application/directorytest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// acme is an organization with an owner (ada), an admin (carol), a member (dave) who is an
// operator on payments, and a ledger admin of payroll (lea). bob has an account elsewhere.
type acme struct {
	dir    *directorytest.Directory
	orgID  string
	grants grantsBySubject
	m      *members
}

func newAcme() acme {
	dir := directorytest.New("kc")
	for _, s := range []string{"ada", "bob", "carol", "dave", "lea"} {
		dir.AddPerson(s, s+"@example.com")
	}
	orgID := dir.AddOrganization("acme")
	for _, s := range []string{"ada", "carol", "dave", "lea"} {
		_ = dir.AddMember(context.Background(), orgID, s)
	}
	grants := grantsBySubject{
		"ada":   {{Role: ledger.RoleOwner}},
		"carol": {{Role: ledger.RoleOrgAdmin}},
		"dave":  {{Role: ledger.RoleMember}, {Ledger: "payments", Role: ledger.RoleOperator}},
		"lea":   {{Role: ledger.RoleMember}, {Ledger: "payroll", Role: ledger.RoleLedgerAdmin}},
	}
	// The seeded organization has no Keycloak ID yet: the first call looks it up.
	orgs := fakeOrgs{"acme": {ID: ledger.ID{15: 9}, Alias: "acme", Name: "Acme", Status: domain.OrganizationActive}}
	now := time.Date(2026, 10, 26, 13, 0, 0, 0, time.UTC)
	m := &members{authz: authorizer{grants: grants}, orgs: orgs, grants: grants, dir: dir, clock: ledger.ClockFunc(func() time.Time { return now })}
	return acme{dir: dir, orgID: orgID, grants: grants, m: m}
}

func roleOf(g grantsBySubject, subject, ledgerName string) ledger.Role {
	for _, gr := range g[subject] {
		if gr.Ledger == ledgerName {
			return gr.Role
		}
	}
	return ""
}

func TestInvite(t *testing.T) {
	t.Run("a person without an account gets one and the setup email", func(t *testing.T) {
		a := newAcme()
		v, err := a.m.Invite(call("carol"), Invite{Email: " New.Person@Example.com ", FirstName: "New", Role: ledger.RoleOrgAdmin})
		if err != nil {
			t.Fatal(err)
		}
		if v.Status != MemberSetupPending || v.Email != "new.person@example.com" || v.Role != ledger.RoleOrgAdmin {
			t.Errorf("Invite = %+v", v)
		}
		if !a.dir.IsMember(a.orgID, v.Subject) {
			t.Error("the new account is not a member")
		}
		if diff := cmp.Diff([]string{"setup " + v.Subject}, a.dir.Emails()); diff != "" {
			t.Errorf("emails (-want +got):\n%s", diff)
		}
		// A retry, after a lost answer, sends the setup email again instead of failing.
		if _, err := a.m.Invite(call("carol"), Invite{Email: "new.person@example.com", Role: ledger.RoleOrgAdmin}); err != nil {
			t.Fatalf("retry: %v", err)
		}
		if len(a.dir.Emails()) != 2 || a.dir.People() != 6 {
			t.Errorf("retry: emails %v, %d people; want the setup email again and no second account", a.dir.Emails(), a.dir.People())
		}
	})

	t.Run("a person with an account gets the invitation", func(t *testing.T) {
		a := newAcme()
		v, err := a.m.Invite(call("carol"), Invite{Email: "bob@example.com"})
		if err != nil {
			t.Fatal(err)
		}
		if v.Status != MemberInvited || v.Role != ledger.RoleMember || roleOf(a.grants, "bob", "") != ledger.RoleMember {
			t.Errorf("Invite = %+v, grants %v", v, a.grants["bob"])
		}
		if a.dir.IsMember(a.orgID, "bob") {
			t.Error("bob is a member before accepting")
		}
		if diff := cmp.Diff([]string{"invite bob"}, a.dir.Emails()); diff != "" {
			t.Errorf("emails (-want +got):\n%s", diff)
		}
	})

	tests := []struct {
		name   string
		caller string
		in     Invite
		want   error
	}{
		{"a member with a role", "carol", Invite{Email: "dave@example.com"}, ledger.ErrAlreadyExists},
		{"an owner by an admin", "carol", Invite{Email: "bob@example.com", Role: ledger.RoleOwner}, ledger.ErrForbidden},
		{"an owner by an owner", "ada", Invite{Email: "bob@example.com", Role: ledger.RoleOwner}, nil},
		{"by a member", "dave", Invite{Email: "bob@example.com"}, ledger.ErrForbidden},
		{"a bad email", "carol", Invite{Email: "Bob <bob@example.com>"}, ledger.ErrInvalid},
		{"a ledger role", "carol", Invite{Email: "bob@example.com", Role: ledger.RoleViewer}, ledger.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newAcme().m.Invite(call(tt.caller), tt.in)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("Invite = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestList(t *testing.T) {
	a := newAcme()
	if _, err := a.m.Invite(call("ada"), Invite{Email: "bob@example.com", Role: ledger.RoleOrgAdmin}); err != nil {
		t.Fatal(err)
	}
	got, err := a.m.List(call("carol"))
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Subject string
		Status  MemberStatus
		Role    ledger.Role
		Ledgers map[string]ledger.Role
		Invited bool
	}
	rows := make([]row, len(got))
	for i, v := range got {
		rows[i] = row{v.Subject, v.Status, v.Role, v.Ledgers, v.Invitation != nil}
	}
	want := []row{
		{"ada", MemberActive, ledger.RoleOwner, map[string]ledger.Role{}, false},
		{"bob", MemberInvited, ledger.RoleOrgAdmin, map[string]ledger.Role{}, true},
		{"carol", MemberActive, ledger.RoleOrgAdmin, map[string]ledger.Role{}, false},
		{"dave", MemberActive, ledger.RoleMember, map[string]ledger.Role{"payments": ledger.RoleOperator}, false},
		{"lea", MemberActive, ledger.RoleMember, map[string]ledger.Role{"payroll": ledger.RoleLedgerAdmin}, false},
	}
	if diff := cmp.Diff(want, rows); diff != "" {
		t.Errorf("List (-want +got):\n%s", diff)
	}
	if _, err := a.m.List(call("dave")); !errors.Is(err, ledger.ErrForbidden) {
		t.Errorf("List by a member = %v, want ErrForbidden", err)
	}
}

func TestOwnerRules(t *testing.T) {
	tests := []struct {
		name string
		do   func(m *members) error
		want error
	}{
		{"an admin cannot demote an owner", func(m *members) error { return m.SetRole(call("carol"), "ada", ledger.RoleMember) }, ledger.ErrForbidden},
		{"an admin cannot promote to owner", func(m *members) error { return m.SetRole(call("carol"), "dave", ledger.RoleOwner) }, ledger.ErrForbidden},
		{"an owner promotes to owner", func(m *members) error { return m.SetRole(call("ada"), "dave", ledger.RoleOwner) }, nil},
		{"the last owner stays", func(m *members) error { return m.SetRole(call("ada"), "ada", ledger.RoleOrgAdmin) }, ledger.ErrConflict},
		{"the last owner cannot be removed", func(m *members) error { return m.Remove(call("ada"), "ada") }, ledger.ErrConflict},
		{"an admin cannot remove an owner", func(m *members) error { return m.Remove(call("carol"), "ada") }, ledger.ErrForbidden},
		{"no role for a stranger", func(m *members) error { return m.SetRole(call("ada"), "bob", ledger.RoleMember) }, ledger.ErrNotFound},
		{"organization roles only", func(m *members) error { return m.SetRole(call("ada"), "dave", ledger.RoleOperator) }, ledger.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.do(newAcme().m)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("= %v, want %v", err, tt.want)
			}
		})
	}
	t.Run("an owner steps down once there is another", func(t *testing.T) {
		a := newAcme()
		if err := a.m.SetRole(call("ada"), "carol", ledger.RoleOwner); err != nil {
			t.Fatal(err)
		}
		if err := a.m.SetRole(call("ada"), "ada", ledger.RoleMember); err != nil {
			t.Fatal(err)
		}
		if roleOf(a.grants, "ada", "") != ledger.RoleMember {
			t.Errorf("ada holds %v", a.grants["ada"])
		}
	})
}

func TestRemove(t *testing.T) {
	a := newAcme()
	if err := a.m.Remove(call("carol"), "dave"); err != nil {
		t.Fatal(err)
	}
	if a.dir.IsMember(a.orgID, "dave") || len(a.grants["dave"]) != 0 {
		t.Errorf("dave: member %v, grants %v; want neither", a.dir.IsMember(a.orgID, "dave"), a.grants["dave"])
	}
	if !hasPerson(a.dir, "dave") {
		t.Error("removing a member deleted the account")
	}
	if err := a.m.Remove(call("carol"), "bob"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("Remove a non-member = %v, want ErrNotFound", err)
	}
}

func TestLedgerRoles(t *testing.T) {
	tests := []struct {
		name    string
		caller  string
		subject string
		ledger  string
		role    ledger.Role
		want    error
	}{
		{"an admin on any ledger", "carol", "dave", "payroll", ledger.RoleViewer, nil},
		{"a ledger admin on their ledger", "lea", "dave", "payroll", ledger.RoleOperator, nil},
		{"a ledger admin on a ledger they cannot see", "lea", "dave", "payments", ledger.RoleViewer, ledger.ErrNotFound},
		{"an operator", "dave", "lea", "payments", ledger.RoleViewer, ledger.ErrForbidden},
		{"a stranger", "carol", "bob", "payments", ledger.RoleViewer, ledger.ErrNotFound},
		{"a ledger that does not exist", "carol", "dave", "nope", ledger.RoleViewer, ledger.ErrNotFound},
		{"an organization role", "carol", "dave", "payments", ledger.RoleOwner, ledger.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAcme()
			err := a.m.SetLedgerRole(call(tt.caller), tt.subject, tt.ledger, tt.role)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("SetLedgerRole = %v, want %v", err, tt.want)
			}
			if tt.want == nil && roleOf(a.grants, tt.subject, tt.ledger) != tt.role {
				t.Errorf("%s on %s holds %v", tt.subject, tt.ledger, a.grants[tt.subject])
			}
		})
	}
	a := newAcme()
	if err := a.m.RevokeLedgerRole(call("carol"), "dave", "payments"); err != nil {
		t.Fatal(err)
	}
	if roleOf(a.grants, "dave", "payments") != "" || roleOf(a.grants, "dave", "") != ledger.RoleMember {
		t.Errorf("dave holds %v; want only the organization role", a.grants["dave"])
	}
}

func TestInvitations(t *testing.T) {
	a := newAcme()
	if _, err := a.m.Invite(call("carol"), Invite{Email: "bob@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := a.m.ResendInvitation(call("carol"), "inv-bob"); err != nil {
		t.Fatal(err)
	}
	if err := a.m.DeleteInvitation(call("carol"), "inv-bob"); err != nil {
		t.Fatal(err)
	}
	if len(a.grants["bob"]) != 0 || len(a.dir.PendingInvitations(a.orgID)) != 0 {
		t.Errorf("after deleting: grants %v, invitations %v", a.grants["bob"], a.dir.PendingInvitations(a.orgID))
	}
	if err := a.m.DeleteInvitation(call("carol"), "inv-bob"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("deleting twice = %v, want ErrNotFound", err)
	}
	if err := a.m.ResendInvitation(context.Background(), "inv-bob"); !errors.Is(err, ledger.ErrUnauthenticated) {
		t.Errorf("anonymous = %v, want ErrUnauthenticated", err)
	}
}

func hasPerson(d *directorytest.Directory, subject string) bool {
	_, ok := d.Person(subject)
	return ok
}
