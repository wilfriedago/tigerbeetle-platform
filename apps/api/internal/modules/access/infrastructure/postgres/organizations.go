package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// ListOrganizations returns up to limit organizations after the alias, in alias order.
func (s *Store) ListOrganizations(ctx context.Context, after string, limit int) ([]domain.Organization, error) {
	rows, err := s.queries.ListOrganizations(ctx, gen.ListOrganizationsParams{After: after, MaxRows: int32(min(limit, 1000))}) //nolint:gosec // bounded above
	if err != nil {
		return nil, fmt.Errorf("postgres: list organizations: %w", platformpg.MapError(err))
	}
	out := make([]domain.Organization, len(rows))
	for i := range rows {
		out[i] = organizationOf(gen.OrganizationByAliasRow(rows[i]))
	}
	return out, nil
}

// SaveOrganization records a new organization, or completes the one with the same alias.
func (s *Store) SaveOrganization(ctx context.Context, o domain.NewOrganization) (domain.Organization, error) {
	keycloakID, err := keycloakUUID(o.KeycloakID)
	if err != nil {
		return domain.Organization{}, err
	}
	row, err := s.queries.SaveOrganization(ctx, gen.SaveOrganizationParams{
		Alias:            o.Alias,
		KeycloakID:       keycloakID,
		Name:             o.Name,
		CreatedByIssuer:  platformpg.OptionalString(o.CreatedBy.Issuer),
		CreatedBySubject: platformpg.OptionalString(o.CreatedBy.Subject),
		CreatedByName:    platformpg.OptionalString(o.CreatedBy.Name),
	})
	if err != nil {
		return domain.Organization{}, fmt.Errorf("postgres: save organization: %w", platformpg.MapError(err))
	}
	return organizationOf(gen.OrganizationByAliasRow(row)), nil
}

// SetKeycloakID records the Keycloak ID of an organization that had none.
func (s *Store) SetKeycloakID(ctx context.Context, alias, keycloakID string) error {
	id, err := keycloakUUID(keycloakID)
	if err != nil {
		return err
	}
	if err := s.queries.SetOrganizationKeycloakID(ctx, gen.SetOrganizationKeycloakIDParams{Alias: alias, KeycloakID: id}); err != nil {
		return fmt.Errorf("postgres: set keycloak id: %w", platformpg.MapError(err))
	}
	return nil
}

// RenameOrganization changes the name of the organization with the alias.
func (s *Store) RenameOrganization(ctx context.Context, alias, name string) (domain.Organization, error) {
	row, err := s.queries.RenameOrganization(ctx, gen.RenameOrganizationParams{Alias: alias, Name: name})
	if err != nil {
		return domain.Organization{}, fmt.Errorf("postgres: rename organization: %w", platformpg.MapError(err))
	}
	return organizationOf(gen.OrganizationByAliasRow(row)), nil
}

// SetOrganizationStatus suspends the organization with the alias for the reason, or reactivates it.
func (s *Store) SetOrganizationStatus(ctx context.Context, alias string, status domain.OrganizationStatus, reason string) (domain.Organization, error) {
	row, err := s.queries.SetOrganizationStatus(ctx, gen.SetOrganizationStatusParams{Alias: alias, Status: string(status), Reason: platformpg.OptionalString(reason)})
	if err != nil {
		return domain.Organization{}, fmt.Errorf("postgres: organization status: %w", platformpg.MapError(err))
	}
	return organizationOf(gen.OrganizationByAliasRow(row)), nil
}

// keycloakUUID parses Keycloak's ID of an organization; "" is NULL. Keycloak's IDs are UUIDs, so
// another value is a fault of the server, not of the caller: a plain error, answered as 500.
func keycloakUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if s == "" {
		return id, nil
	}
	if err := id.Scan(s); err != nil {
		return id, fmt.Errorf("postgres: keycloak id %q is not a UUID: %w", s, err)
	}
	return id, nil
}
