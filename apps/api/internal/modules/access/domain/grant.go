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

// GrantRepository reads grants. The organization is the tenant of the context.
type GrantRepository interface {
	// GrantsOf returns the grants of one principal in the organization of the call.
	GrantsOf(ctx context.Context, issuer, subject string) ([]Grant, error)
}
