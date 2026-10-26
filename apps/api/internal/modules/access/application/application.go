// Package application holds the use cases of the access module (ADR-0027, ADR-0028): finding the
// organization a call is made for, and, in later phases, grants, members and credentials.
// usecases.go exports them as interfaces.
package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// Deps are the access module's ports.
type Deps struct {
	Organizations domain.OrganizationRepository
	Grants        domain.GrantRepository
	Sessions      domain.SessionRepository
	// Directory is Keycloak's admin API. Without it, organizations and members answer
	// ledger.ErrUnavailable.
	Directory domain.Directory
	Clock     ledger.Clock
}

// SessionDeps are what the console's sign-in needs; without them the module has no Sessions.
type SessionDeps struct {
	IdentityProvider IdentityProvider
	Tokens           TokenVerifier
	Sealer           Sealer
}

// Application is the access module's use cases, built by New.
type Application struct {
	OrganizationQueries OrganizationQueries
	ViewerQueries       ViewerQueries
	// Authorizer answers the other modules' Authorizer ports.
	Authorizer Authorizer
	// PlatformOrganizations are the staff's.
	PlatformOrganizations PlatformOrganizations
	Members               Members
}

// New builds the module on its ports.
func New(d Deps) Application {
	dir := d.Directory
	if dir == nil {
		dir = noDirectory{}
	}
	authz := authorizer{grants: d.Grants}
	m := &members{authz: authz, orgs: d.Organizations, grants: d.Grants, dir: dir, clock: d.Clock}
	return Application{
		OrganizationQueries:   organizationQueries{orgs: d.Organizations},
		ViewerQueries:         viewerQueries{orgs: d.Organizations, grants: d.Grants},
		Authorizer:            authz,
		PlatformOrganizations: platformOrganizations{authz: authz, orgs: d.Organizations, grants: d.Grants, members: m},
		Members:               m,
	}
}

// NewSessions builds the console's sign-in (ADR-0030).
func NewSessions(d Deps, s SessionDeps) Sessions {
	return &sessions{idp: s.IdentityProvider, tokens: s.Tokens, sealer: s.Sealer, store: d.Sessions, clock: d.Clock}
}

type organizationQueries struct {
	orgs domain.OrganizationRepository
}

// Tenant resolves the organization a call names. A suspended organization answers 403
// organization-suspended (ADR-0027).
func (q organizationQueries) Tenant(ctx context.Context, alias string) (ledger.Tenant, error) {
	org, err := q.orgs.OrganizationByAlias(ctx, alias)
	if err != nil {
		return ledger.Tenant{}, fmt.Errorf("access: organization %q: %w", alias, err)
	}
	if org.Status == domain.OrganizationSuspended {
		return ledger.Tenant{}, ledger.NewProblem(ledger.ErrForbidden, "organization-suspended", "the organization "+alias+" is suspended")
	}
	return org.Tenant(), nil
}
