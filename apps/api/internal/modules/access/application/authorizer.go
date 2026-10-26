package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// authorizer decides with the grants of the caller in the organization of the call (ADR-0027).
// It reads them on every call, without a cache, so a revoked grant stops working at once.
type authorizer struct {
	grants domain.GrantRepository
}

// access is what the caller holds in the organization of the call.
type access struct {
	org     ledger.Role
	ledgers map[string]ledger.Role
}

func (a authorizer) accessOf(ctx context.Context, p ledger.Principal) (access, error) {
	grants, err := a.grants.GrantsOf(ctx, p.Issuer, p.Subject)
	if err != nil {
		return access{}, fmt.Errorf("access: grants: %w", err)
	}
	out := access{ledgers: map[string]ledger.Role{}}
	for _, g := range grants {
		if g.Ledger == "" {
			out.org = g.Role
		} else {
			out.ledgers[g.Ledger] = g.Role
		}
	}
	return out, nil
}

// Require returns nil when the caller holds the permission: on the ledger for a ledger
// permission, in the organization of the call otherwise. A caller who holds nothing on the ledger
// gets ledger.ErrNotFound, the answer for a ledger that does not exist, so ledger names are not
// disclosed; a caller who can see it but lacks the permission gets ledger.ErrForbidden.
func (a authorizer) Require(ctx context.Context, perm ledger.Permission, ledgerName string) error {
	p, ok := ledger.PrincipalFrom(ctx)
	if !ok {
		return ledger.ErrUnauthenticated
	}
	if p.Kind == ledger.KindSystem {
		return nil
	}
	if perm == ledger.PermPlatformOrganizations {
		if p.Staff {
			return nil
		}
		return forbidden(perm)
	}
	if _, err := ledger.RequireTenant(ctx); err != nil {
		return err
	}
	acc, err := a.accessOf(ctx, p)
	if err != nil {
		return err
	}
	if ledger.OrganizationRoleAllows(acc.org, perm) {
		return nil
	}
	if !perm.OnLedger() {
		return forbidden(perm)
	}
	role, holds := acc.ledgers[ledgerName]
	if !holds {
		return fmt.Errorf("ledger %q: %w", ledgerName, ledger.ErrNotFound)
	}
	if !ledger.LedgerRoleAllows(role, perm) {
		return forbidden(perm)
	}
	return nil
}

// VisibleLedgers returns all true when the caller may read every ledger of the organization,
// and otherwise the names of the ledgers it holds a role on.
func (a authorizer) VisibleLedgers(ctx context.Context) (all bool, names []string, err error) {
	p, ok := ledger.PrincipalFrom(ctx)
	if !ok {
		return false, nil, ledger.ErrUnauthenticated
	}
	if p.Kind == ledger.KindSystem {
		return true, nil, nil
	}
	if _, err := ledger.RequireTenant(ctx); err != nil {
		return false, nil, err
	}
	acc, err := a.accessOf(ctx, p)
	if err != nil {
		return false, nil, err
	}
	if ledger.OrganizationRoleAllows(acc.org, ledger.PermLedgerRead) {
		return true, nil, nil
	}
	names = make([]string, 0, len(acc.ledgers))
	for name := range acc.ledgers {
		names = append(names, name)
	}
	return false, names, nil
}

func forbidden(perm ledger.Permission) error {
	return ledger.NewProblem(ledger.ErrForbidden, "forbidden", "this needs the permission "+string(perm))
}
