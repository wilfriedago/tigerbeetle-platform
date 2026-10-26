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
	Email         string
	Staff         bool
	Organizations []ViewerOrganization
	// Role is the caller's organization role in the organization of the call, "" when the call
	// names none or the caller holds none.
	Role ledger.Role
	// Ledgers maps ledger names to the caller's role on them, in the organization of the call.
	Ledgers map[string]ledger.Role
}

// ViewerOrganization is an organization the caller belongs to, with their roles there.
type ViewerOrganization struct {
	Alias string
	Name  string
	// Role is the caller's organization role there, "" for none.
	Role ledger.Role
	// Ledgers maps ledger names to the caller's role on them there.
	Ledgers map[string]ledger.Role
	// Suspended organizations are listed so the console can say why they refuse every call.
	Status           domain.OrganizationStatus
	SuspensionReason string
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
	v := Viewer{Subject: p.Subject, Name: p.Name, Email: p.Email, Staff: p.Staff, Ledgers: map[string]ledger.Role{}}
	if len(p.Organizations) > 0 {
		orgs, err := q.orgs.OrganizationsByAlias(ctx, p.Organizations)
		if err != nil {
			return Viewer{}, fmt.Errorf("access: organizations: %w", err)
		}
		across, err := q.grants.GrantsAcross(ctx, p.Issuer, p.Subject)
		if err != nil {
			return Viewer{}, fmt.Errorf("access: grants: %w", err)
		}
		for i := range orgs {
			o := &orgs[i]
			vo := ViewerOrganization{
				Alias: o.Alias, Name: o.Name, Ledgers: map[string]ledger.Role{},
				Status: o.Status, SuspensionReason: o.SuspensionReason,
			}
			for _, g := range across {
				switch {
				case g.Alias != o.Alias:
				case g.Ledger == "":
					vo.Role = g.Role
				default:
					vo.Ledgers[g.Ledger] = g.Role
				}
			}
			v.Organizations = append(v.Organizations, vo)
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
