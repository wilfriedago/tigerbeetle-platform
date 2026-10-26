// Package postgres implements the access module's repositories on the pool of
// internal/platform/postgres. Its queries, in queries/, touch only the tables the module owns;
// sqlc generates gen/ from them.
package postgres

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// Store implements the access module's repositories: organizations, grants and sessions.
type Store struct {
	queries *gen.Queries
}

// NewStore builds the store on the process's pool.
func NewStore(db *platformpg.DB) *Store {
	return &Store{queries: gen.New(db.Pool())}
}

// GrantsOf returns the grants of a principal in the organization of the call.
func (s *Store) GrantsOf(ctx context.Context, issuer, subject string) ([]domain.Grant, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.GrantsOf(ctx, gen.GrantsOfParams{OrganizationID: org, Issuer: issuer, Subject: subject})
	if err != nil {
		return nil, fmt.Errorf("postgres: grants: %w", platformpg.MapError(err))
	}
	out := make([]domain.Grant, len(rows))
	for i, r := range rows {
		out[i] = domain.Grant{Ledger: platformpg.StringOf(r.Ledger), Role: ledger.Role(r.Role)}
	}
	return out, nil
}

// OrganizationByAlias loads an organization by its alias.
func (s *Store) OrganizationByAlias(ctx context.Context, alias string) (domain.Organization, error) {
	row, err := s.queries.OrganizationByAlias(ctx, alias)
	if err != nil {
		return domain.Organization{}, fmt.Errorf("postgres: organization by alias: %w", platformpg.MapError(err))
	}
	return organizationOf(row), nil
}

// organizationOf converts a row. Every organization query selects the same columns, so sqlc's
// row types share one shape and convert to OrganizationByAliasRow.
func organizationOf(row gen.OrganizationByAliasRow) domain.Organization {
	org := domain.Organization{
		ID:        platformpg.IDOf(row.ID),
		Alias:     row.Alias,
		Name:      row.Name,
		Status:    domain.OrganizationStatus(row.Status),
		CreatedAt: platformpg.TimeOf(row.CreatedAt),
	}
	if row.KeycloakID.Valid {
		org.KeycloakID = row.KeycloakID.String()
	}
	org.CreatedByName = platformpg.StringOf(row.CreatedByName)
	org.SuspensionReason = platformpg.StringOf(row.SuspensionReason)
	return org
}
