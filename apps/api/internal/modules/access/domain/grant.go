package domain

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Grant is a role a principal holds in the organization of the call, or on one of its ledgers
// (ADR-0028).
type Grant struct {
	// Ledger is "" for a role in the whole organization.
	Ledger string
	Role   ledger.Role
}

// Holder is a principal that holds grants: a person, or in phase 6 an API key or machine client.
type Holder struct {
	Issuer  string
	Subject string
}

// HeldGrant is a grant with its holder.
type HeldGrant struct {
	Holder
	Grant
}

// GrantRepository stores grants. The organization is the tenant of the context.
type GrantRepository interface {
	// GrantsOf returns the grants of one principal in the organization of the call.
	GrantsOf(ctx context.Context, issuer, subject string) ([]Grant, error)
	// GrantsIn returns every grant in the organization of the call.
	GrantsIn(ctx context.Context) ([]HeldGrant, error)
	// PutGrant gives the holder the role, replacing the one it held in the organization or on the
	// ledger. A ledger that does not exist is ledger.ErrNotFound.
	PutGrant(ctx context.Context, g HeldGrant, by ledger.Principal) error
	// RevokeGrant removes the holder's role on the ledger, or in the organization for "";
	// ledger.ErrNotFound when it held none.
	RevokeGrant(ctx context.Context, h Holder, ledgerName string) error
	// RevokeGrants removes every role of the holder in the organization.
	RevokeGrants(ctx context.Context, h Holder) error
	// CountOwners counts the owners of the organization.
	CountOwners(ctx context.Context) (int, error)
}
