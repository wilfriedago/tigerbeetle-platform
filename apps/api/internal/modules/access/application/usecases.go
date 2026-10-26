package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// OrganizationQueries finds organizations.
type OrganizationQueries interface {
	// Tenant returns the tenant of the organization with the alias: ledger.ErrNotFound when there
	// is none, ledger.ErrForbidden (organization-suspended) when it is suspended.
	Tenant(ctx context.Context, alias string) (ledger.Tenant, error)
}
