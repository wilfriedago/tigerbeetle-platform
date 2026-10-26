package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Authorizer decides whether the caller of a call may do something (ADR-0027).
type Authorizer interface {
	Require(ctx context.Context, perm ledger.Permission, ledgerName string) error
	VisibleLedgers(ctx context.Context) (all bool, names []string, err error)
}

// OrganizationQueries finds organizations.
type OrganizationQueries interface {
	// Tenant returns the tenant of the organization with the alias: ledger.ErrNotFound when there
	// is none, ledger.ErrForbidden (organization-suspended) when it is suspended.
	Tenant(ctx context.Context, alias string) (ledger.Tenant, error)
}
