package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// Authorizer decides whether the caller of a call may do something (ADR-0028).
type Authorizer interface {
	Require(ctx context.Context, perm ledger.Permission, ledgerName string) error
	VisibleLedgers(ctx context.Context) (all bool, names []string, err error)
}

// Sessions are the console's sign-in through the backend-for-frontend (ADR-0030).
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

// PlatformOrganizations are the staff's use cases (ADR-0027). Each needs platform.organizations.
type PlatformOrganizations interface {
	List(ctx context.Context, cursor string, limit int) ([]OrganizationView, string, error)
	Get(ctx context.Context, alias string) (OrganizationView, error)
	Create(ctx context.Context, in CreateOrganization) (OrganizationView, error)
	Rename(ctx context.Context, alias, name string) (OrganizationView, error)
	SetStatus(ctx context.Context, alias string, status domain.OrganizationStatus, reason string) (OrganizationView, error)
}

// Members are the people of the organization of the call and their roles (ADR-0027, ADR-0028).
type Members interface {
	List(ctx context.Context) ([]MemberView, error)
	Invite(ctx context.Context, in Invite) (MemberView, error)
	SetRole(ctx context.Context, subject string, role ledger.Role) error
	Remove(ctx context.Context, subject string) error
	SetLedgerRole(ctx context.Context, subject, ledgerName string, role ledger.Role) error
	RevokeLedgerRole(ctx context.Context, subject, ledgerName string) error
	ResendInvitation(ctx context.Context, id string) error
	DeleteInvitation(ctx context.Context, id string) error
}
