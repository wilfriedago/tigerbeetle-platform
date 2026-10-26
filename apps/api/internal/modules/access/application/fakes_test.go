package application

import (
	"context"
	"slices"
	"strings"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// The rest of domain.GrantRepository on grantsBySubject. Issuers are ignored: every test person
// comes from the issuer "kc", and the ledgers payments and payroll exist.

func (g grantsBySubject) GrantsIn(context.Context) ([]domain.HeldGrant, error) {
	var out []domain.HeldGrant
	for subject, grants := range g {
		for _, gr := range grants {
			out = append(out, domain.HeldGrant{Holder: domain.Holder{Issuer: "kc", Subject: subject}, Grant: gr})
		}
	}
	return out, nil
}

func (g grantsBySubject) PutGrant(_ context.Context, hg domain.HeldGrant, _ ledger.Principal) error {
	if hg.Ledger != "" && hg.Ledger != "payments" && hg.Ledger != "payroll" {
		return ledger.ErrNotFound
	}
	kept := slices.DeleteFunc(g[hg.Subject], func(x domain.Grant) bool { return x.Ledger == hg.Ledger })
	g[hg.Subject] = append(kept, hg.Grant)
	return nil
}

func (g grantsBySubject) RevokeGrant(_ context.Context, h domain.Holder, ledgerName string) error {
	before := len(g[h.Subject])
	g[h.Subject] = slices.DeleteFunc(g[h.Subject], func(x domain.Grant) bool { return x.Ledger == ledgerName })
	if len(g[h.Subject]) == before {
		return ledger.ErrNotFound
	}
	return nil
}

func (g grantsBySubject) RevokeGrants(_ context.Context, h domain.Holder) error {
	delete(g, h.Subject)
	return nil
}

func (g grantsBySubject) CountOwners(context.Context) (int, error) {
	n := 0
	for _, grants := range g {
		for _, gr := range grants {
			if gr.Ledger == "" && gr.Role == ledger.RoleOwner {
				n++
			}
		}
	}
	return n, nil
}

// fakeOrgs is domain.OrganizationRepository in memory.
type fakeOrgs map[string]domain.Organization

func (f fakeOrgs) OrganizationByAlias(_ context.Context, alias string) (domain.Organization, error) {
	o, ok := f[alias]
	if !ok {
		return o, ledger.ErrNotFound
	}
	return o, nil
}

func (f fakeOrgs) OrganizationsByAlias(_ context.Context, aliases []string) ([]domain.Organization, error) {
	var out []domain.Organization
	for _, a := range aliases {
		if o, ok := f[a]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f fakeOrgs) ListOrganizations(_ context.Context, after string, limit int) ([]domain.Organization, error) {
	var out []domain.Organization
	for _, o := range f {
		if o.Alias > after {
			out = append(out, o)
		}
	}
	slices.SortFunc(out, func(a, b domain.Organization) int { return strings.Compare(a.Alias, b.Alias) })
	return out[:min(limit, len(out))], nil
}

func (f fakeOrgs) SaveOrganization(_ context.Context, n domain.NewOrganization) (domain.Organization, error) {
	if o, ok := f[n.Alias]; ok {
		if o.KeycloakID == "" {
			o.KeycloakID = n.KeycloakID
			f[n.Alias] = o
		}
		return o, nil
	}
	o := domain.Organization{ID: ledger.ID{15: byte(len(f) + 1)}, Alias: n.Alias, Name: n.Name, Status: domain.OrganizationActive, KeycloakID: n.KeycloakID}
	f[n.Alias] = o
	return o, nil
}

func (f fakeOrgs) SetKeycloakID(_ context.Context, alias, id string) error {
	o := f[alias]
	o.KeycloakID = id
	f[alias] = o
	return nil
}

func (f fakeOrgs) RenameOrganization(_ context.Context, alias, name string) (domain.Organization, error) {
	o, ok := f[alias]
	if !ok {
		return o, ledger.ErrNotFound
	}
	o.Name = name
	f[alias] = o
	return o, nil
}

func (f fakeOrgs) SetOrganizationStatus(_ context.Context, alias string, s domain.OrganizationStatus) (domain.Organization, error) {
	o, ok := f[alias]
	if !ok {
		return o, ledger.ErrNotFound
	}
	o.Status = s
	f[alias] = o
	return o, nil
}
