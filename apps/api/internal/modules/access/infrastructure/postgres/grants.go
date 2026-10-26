package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// GrantsIn returns every grant in the organization of the call.
func (s *Store) GrantsIn(ctx context.Context) ([]domain.HeldGrant, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.GrantsIn(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("postgres: grants in organization: %w", platformpg.MapError(err))
	}
	out := make([]domain.HeldGrant, len(rows))
	for i, r := range rows {
		out[i] = domain.HeldGrant{
			Holder: domain.Holder{Issuer: r.Issuer, Subject: r.Subject},
			Grant:  domain.Grant{Ledger: platformpg.StringOf(r.Ledger), Role: ledger.Role(r.Role)},
		}
	}
	return out, nil
}

// PutGrant gives the holder the role, replacing the one it held. A ledger of another organization
// or none fails the composite foreign key: ledger.ErrNotFound.
func (s *Store) PutGrant(ctx context.Context, g domain.HeldGrant, by ledger.Principal) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	err = s.queries.PutGrant(ctx, gen.PutGrantParams{
		OrganizationID:   org,
		Issuer:           g.Issuer,
		Subject:          g.Subject,
		Role:             string(g.Role),
		Ledger:           platformpg.OptionalString(g.Ledger),
		GrantedByIssuer:  platformpg.OptionalString(by.Issuer),
		GrantedBySubject: platformpg.OptionalString(by.Subject),
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation {
		return fmt.Errorf("postgres: put grant: ledger %q: %w", g.Ledger, ledger.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("postgres: put grant: %w", platformpg.MapError(err))
	}
	return nil
}

// foreignKeyViolation is Postgres's SQLSTATE for a foreign key that points at no row.
const foreignKeyViolation = "23503"

// RevokeGrant removes the holder's role on the ledger, or in the organization for "".
func (s *Store) RevokeGrant(ctx context.Context, h domain.Holder, ledgerName string) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	n, err := s.queries.RevokeGrant(ctx, gen.RevokeGrantParams{OrganizationID: org, Issuer: h.Issuer, Subject: h.Subject, Ledger: &ledgerName})
	if err != nil {
		return fmt.Errorf("postgres: revoke grant: %w", platformpg.MapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: revoke grant: %w", ledger.ErrNotFound)
	}
	return nil
}

// RevokeGrants removes every role of the holder in the organization.
func (s *Store) RevokeGrants(ctx context.Context, h domain.Holder) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	if err := s.queries.RevokeGrants(ctx, gen.RevokeGrantsParams{OrganizationID: org, Issuer: h.Issuer, Subject: h.Subject}); err != nil {
		return fmt.Errorf("postgres: revoke grants: %w", platformpg.MapError(err))
	}
	return nil
}

// CountOwners counts the owners of the organization.
func (s *Store) CountOwners(ctx context.Context) (int, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return 0, err
	}
	n, err := s.queries.CountOwners(ctx, org)
	if err != nil {
		return 0, fmt.Errorf("postgres: count owners: %w", platformpg.MapError(err))
	}
	return int(n), nil
}
