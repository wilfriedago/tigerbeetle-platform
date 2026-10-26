// Package application holds the use cases of the access module (ADR-0026, ADR-0027): finding the
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
}

// Application is the access module's use cases, built by New.
type Application struct {
	OrganizationQueries OrganizationQueries
}

// New builds the module on its ports.
func New(d Deps) Application {
	return Application{OrganizationQueries: organizationQueries{orgs: d.Organizations}}
}

type organizationQueries struct {
	orgs domain.OrganizationRepository
}

// Tenant resolves the organization a call names. A suspended organization answers 403
// organization-suspended (ADR-0026).
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
