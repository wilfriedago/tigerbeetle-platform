package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// Viewer is the signed-in caller as the console shows it: who they are, the organizations they
// belong to, and their roles in the organization of the call.
type Viewer struct {
	Subject       string
	Name          string
	Staff         bool
	Organizations []ViewerOrganization
	// Role is the caller's organization role in the organization of the call, "" when the call
	// names none or the caller holds none.
	Role ledger.Role
	// Ledgers maps ledger names to the caller's role on them, in the organization of the call.
	Ledgers map[string]ledger.Role
}

// ViewerOrganization is an organization the caller belongs to.
type ViewerOrganization struct {
	Alias string
	Name  string
}

type viewerQueries struct {
	orgs   domain.OrganizationRepository
	grants domain.GrantRepository
}

// Viewer describes the caller of the call.
func (q viewerQueries) Viewer(ctx context.Context) (Viewer, error) {
	p, ok := ledger.PrincipalFrom(ctx)
	if !ok {
		return Viewer{}, ledger.ErrUnauthenticated
	}
	v := Viewer{Subject: p.Subject, Name: p.Name, Staff: p.Staff, Ledgers: map[string]ledger.Role{}}
	if len(p.Organizations) > 0 {
		orgs, err := q.orgs.OrganizationsByAlias(ctx, p.Organizations)
		if err != nil {
			return Viewer{}, fmt.Errorf("access: organizations: %w", err)
		}
		for _, o := range orgs {
			if o.Status == domain.OrganizationActive {
				v.Organizations = append(v.Organizations, ViewerOrganization{Alias: o.Alias, Name: o.Name})
			}
		}
		sort.Slice(v.Organizations, func(i, j int) bool { return v.Organizations[i].Name < v.Organizations[j].Name })
	}
	if _, ok := ledger.TenantFrom(ctx); ok {
		grants, err := q.grants.GrantsOf(ctx, p.Issuer, p.Subject)
		if err != nil {
			return Viewer{}, fmt.Errorf("access: grants: %w", err)
		}
		for _, g := range grants {
			if g.Ledger == "" {
				v.Role = g.Role
			} else {
				v.Ledgers[g.Ledger] = g.Role
			}
		}
	}
	return v, nil
}
