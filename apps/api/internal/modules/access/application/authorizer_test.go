package application

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// grantsBySubject holds the grants of the organization of every call.
type grantsBySubject map[string][]domain.Grant

func (g grantsBySubject) GrantsOf(_ context.Context, _, subject string) ([]domain.Grant, error) {
	return g[subject], nil
}

func call(subject string) context.Context {
	ctx := ledger.WithPrincipal(context.Background(), ledger.Principal{Issuer: "kc", Subject: subject, Kind: ledger.KindPerson})
	return ledger.WithTenant(ctx, ledger.Tenant{ID: ledger.ID{15: 9}, Alias: "acme"})
}

func TestRequire(t *testing.T) {
	auth := authorizer{grants: grantsBySubject{
		"owner":    {{Role: ledger.RoleOwner}},
		"admin":    {{Role: ledger.RoleOrgAdmin}},
		"member":   {{Role: ledger.RoleMember}, {Ledger: "payments", Role: ledger.RoleViewer}},
		"operator": {{Role: ledger.RoleMember}, {Ledger: "payments", Role: ledger.RoleOperator}},
		"ledgerad": {{Role: ledger.RoleMember}, {Ledger: "payments", Role: ledger.RoleLedgerAdmin}},
	}}
	tests := []struct {
		subject string
		perm    ledger.Permission
		ledger  string
		want    error
	}{
		{"owner", ledger.PermMoneyWrite, "anything", nil},
		{"owner", ledger.PermOrgMembers, "", nil},
		{"admin", ledger.PermLedgerConfigure, "payments", nil},
		{"admin", ledger.PermOrgLedgers, "", nil},
		{"member", ledger.PermLedgerRead, "payments", nil},
		{"member", ledger.PermMoneyWrite, "payments", ledger.ErrForbidden},
		{"member", ledger.PermLedgerRead, "payroll", ledger.ErrNotFound},
		{"member", ledger.PermOrgLedgers, "", ledger.ErrForbidden},
		{"operator", ledger.PermMoneyWrite, "payments", nil},
		{"operator", ledger.PermMetadataWrite, "payments", nil},
		{"operator", ledger.PermLedgerConfigure, "payments", ledger.ErrForbidden},
		{"ledgerad", ledger.PermLedgerConfigure, "payments", nil},
		{"ledgerad", ledger.PermLedgerMembers, "payments", nil},
		{"ledgerad", ledger.PermOrgOperate, "", ledger.ErrForbidden},
		{"stranger", ledger.PermLedgerRead, "payments", ledger.ErrNotFound},
		{"stranger", ledger.PermOrgOperate, "", ledger.ErrForbidden},
		{"admin", ledger.PermPlatformOrganizations, "", ledger.ErrForbidden},
	}
	for _, tt := range tests {
		err := auth.Require(call(tt.subject), tt.perm, tt.ledger)
		if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
			t.Errorf("%s %s on %q = %v, want %v", tt.subject, tt.perm, tt.ledger, err, tt.want)
		}
	}
}

func TestRequireSpecialCallers(t *testing.T) {
	auth := authorizer{grants: grantsBySubject{}}
	if err := auth.Require(context.Background(), ledger.PermLedgerRead, "payments"); !errors.Is(err, ledger.ErrUnauthenticated) {
		t.Errorf("anonymous = %v, want ErrUnauthenticated", err)
	}
	if err := auth.Require(ledger.WithPrincipal(context.Background(), ledger.System), ledger.PermMoneyWrite, "payments"); err != nil {
		t.Errorf("system = %v, want nil", err)
	}
	staff := ledger.WithPrincipal(context.Background(), ledger.Principal{Issuer: "kc", Subject: "ops", Staff: true})
	if err := auth.Require(staff, ledger.PermPlatformOrganizations, ""); err != nil {
		t.Errorf("staff managing organizations = %v, want nil", err)
	}
	if err := auth.Require(ledger.WithTenant(staff, ledger.Tenant{ID: ledger.ID{15: 9}}), ledger.PermLedgerRead, "payments"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("staff reading a tenant's ledger = %v, want ErrNotFound", err)
	}
	noTenant := ledger.WithPrincipal(context.Background(), ledger.Principal{Issuer: "kc", Subject: "x"})
	if err := auth.Require(noTenant, ledger.PermLedgerRead, "payments"); !errors.Is(err, ledger.ErrInvalid) {
		t.Errorf("no organization = %v, want ErrInvalid (organization-required)", err)
	}
}

func TestVisibleLedgers(t *testing.T) {
	auth := authorizer{grants: grantsBySubject{
		"admin":  {{Role: ledger.RoleOrgAdmin}},
		"member": {{Role: ledger.RoleMember}, {Ledger: "payments", Role: ledger.RoleViewer}, {Ledger: "payroll", Role: ledger.RoleOperator}},
	}}
	all, _, err := auth.VisibleLedgers(call("admin"))
	if err != nil || !all {
		t.Fatalf("admin sees all = %v, %v", all, err)
	}
	all, names, err := auth.VisibleLedgers(call("member"))
	slices.Sort(names)
	if err != nil || all || !slices.Equal(names, []string{"payments", "payroll"}) {
		t.Fatalf("member sees %v %v, %v", all, names, err)
	}
	all, names, err = auth.VisibleLedgers(call("stranger"))
	if err != nil || all || len(names) != 0 {
		t.Fatalf("stranger sees %v %v, %v", all, names, err)
	}
}
