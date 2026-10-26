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

// Sessions are the console's sign-in through the backend-for-frontend (ADR-0029).
type Sessions interface {
	Begin(returnTo string) (authURL, cookie string)
	Complete(ctx context.Context, cookie, code, state string) (sessionID, returnTo string, err error)
	AccessToken(ctx context.Context, sessionID string) (string, error)
	End(ctx context.Context, sessionID, postLogoutRedirect string) (logoutURL string, err error)
}

// ViewerQueries describes the caller.
type ViewerQueries interface {
	Viewer(ctx context.Context) (Viewer, error)
}

// OrganizationQueries finds organizations.
type OrganizationQueries interface {
	// Tenant returns the tenant of the organization with the alias: ledger.ErrNotFound when there
	// is none, ledger.ErrForbidden (organization-suspended) when it is suspended.
	Tenant(ctx context.Context, alias string) (ledger.Tenant, error)
}
