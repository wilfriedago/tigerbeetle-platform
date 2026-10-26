package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// CreateAPIKey records a key in the organization of the call; a name taken by an active key is
// ledger.ErrAlreadyExists.
func (s *Store) CreateAPIKey(ctx context.Context, k domain.APIKey, by ledger.Principal) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	err = s.queries.CreateAPIKey(ctx, gen.CreateAPIKeyParams{
		ID: k.ID, OrganizationID: org, Name: k.Name, SecretHash: k.SecretHash,
		ExpiresAt:        platformpg.OptionalTimestamptz(k.ExpiresAt),
		CreatedByIssuer:  platformpg.OptionalString(by.Issuer),
		CreatedBySubject: platformpg.OptionalString(by.Subject),
		CreatedByName:    platformpg.OptionalString(by.Name),
	})
	if err != nil {
		return fmt.Errorf("postgres: create api key: %w", platformpg.MapError(err))
	}
	return nil
}

// APIKeys lists the keys of the organization, active ones first.
func (s *Store) APIKeys(ctx context.Context) ([]domain.APIKey, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.APIKeys(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("postgres: api keys: %w", platformpg.MapError(err))
	}
	out := make([]domain.APIKey, len(rows))
	for i := range rows {
		out[i] = apiKeyOf(gen.APIKeyRow(rows[i]))
	}
	return out, nil
}

// APIKey loads one key of the organization.
func (s *Store) APIKey(ctx context.Context, id string) (domain.APIKey, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return domain.APIKey{}, err
	}
	row, err := s.queries.APIKey(ctx, gen.APIKeyParams{OrganizationID: org, ID: id})
	if err != nil {
		return domain.APIKey{}, fmt.Errorf("postgres: api key %q: %w", id, platformpg.MapError(err))
	}
	return apiKeyOf(row), nil
}

// RevokeAPIKey marks an active key revoked.
func (s *Store) RevokeAPIKey(ctx context.Context, id string, at time.Time) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	n, err := s.queries.RevokeAPIKey(ctx, gen.RevokeAPIKeyParams{At: platformpg.Timestamptz(at), OrganizationID: org, ID: id})
	if err != nil {
		return fmt.Errorf("postgres: revoke api key: %w", platformpg.MapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: revoke api key %q: %w", id, ledger.ErrNotFound)
	}
	return nil
}

// APIKeyForAuthentication finds a key by ID in any organization.
func (s *Store) APIKeyForAuthentication(ctx context.Context, id string) (domain.KeyForAuthentication, error) {
	row, err := s.queries.APIKeyForAuthentication(ctx, id)
	if err != nil {
		return domain.KeyForAuthentication{}, fmt.Errorf("postgres: api key: %w", platformpg.MapError(err))
	}
	return domain.KeyForAuthentication{
		Name: row.Name, SecretHash: row.SecretHash,
		ExpiresAt: platformpg.OptionalTime(row.ExpiresAt), RevokedAt: platformpg.OptionalTime(row.RevokedAt),
		Organization: tenantOrganization(row.OrganizationID, row.Alias, row.Status, row.SuspensionReason),
	}, nil
}

// TouchAPIKey records a use, at most once a minute.
func (s *Store) TouchAPIKey(ctx context.Context, id string, at time.Time) error {
	if err := s.queries.TouchAPIKey(ctx, gen.TouchAPIKeyParams{At: platformpg.Timestamptz(at), ID: id}); err != nil {
		return fmt.Errorf("postgres: touch api key: %w", platformpg.MapError(err))
	}
	return nil
}

// CreateMachineClient records a client in the organization of the call.
func (s *Store) CreateMachineClient(ctx context.Context, c domain.MachineClient, by ledger.Principal) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	keycloakID, err := keycloakUUID(c.KeycloakID)
	if err != nil {
		return err
	}
	err = s.queries.CreateMachineClient(ctx, gen.CreateMachineClientParams{
		ClientID: c.ClientID, OrganizationID: org, KeycloakID: keycloakID, Subject: c.Subject, Name: c.Name,
		CreatedByIssuer:  platformpg.OptionalString(by.Issuer),
		CreatedBySubject: platformpg.OptionalString(by.Subject),
		CreatedByName:    platformpg.OptionalString(by.Name),
	})
	if err != nil {
		return fmt.Errorf("postgres: create machine client: %w", platformpg.MapError(err))
	}
	return nil
}

// MachineClients lists the clients of the organization, active ones first.
func (s *Store) MachineClients(ctx context.Context) ([]domain.MachineClient, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.MachineClients(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("postgres: machine clients: %w", platformpg.MapError(err))
	}
	out := make([]domain.MachineClient, len(rows))
	for i := range rows {
		out[i] = machineClientOf(gen.MachineClientRow(rows[i]))
	}
	return out, nil
}

// MachineClient loads one client of the organization.
func (s *Store) MachineClient(ctx context.Context, clientID string) (domain.MachineClient, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return domain.MachineClient{}, err
	}
	row, err := s.queries.MachineClient(ctx, gen.MachineClientParams{OrganizationID: org, ClientID: clientID})
	if err != nil {
		return domain.MachineClient{}, fmt.Errorf("postgres: machine client %q: %w", clientID, platformpg.MapError(err))
	}
	return machineClientOf(row), nil
}

// SetSecretSetAt records a new secret for an active client.
func (s *Store) SetSecretSetAt(ctx context.Context, clientID string, at time.Time) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	n, err := s.queries.SetMachineClientSecretSetAt(ctx, gen.SetMachineClientSecretSetAtParams{At: platformpg.Timestamptz(at), OrganizationID: org, ClientID: clientID})
	if err != nil {
		return fmt.Errorf("postgres: machine client secret: %w", platformpg.MapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: machine client %q: %w", clientID, ledger.ErrNotFound)
	}
	return nil
}

// DeleteMachineClient marks an active client deleted.
func (s *Store) DeleteMachineClient(ctx context.Context, clientID string, at time.Time) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	n, err := s.queries.DeleteMachineClient(ctx, gen.DeleteMachineClientParams{At: platformpg.Timestamptz(at), OrganizationID: org, ClientID: clientID})
	if err != nil {
		return fmt.Errorf("postgres: delete machine client: %w", platformpg.MapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: machine client %q: %w", clientID, ledger.ErrNotFound)
	}
	return nil
}

// MachineClientForAuthentication finds a client by ID in any organization.
func (s *Store) MachineClientForAuthentication(ctx context.Context, clientID string) (domain.ClientForAuthentication, error) {
	row, err := s.queries.MachineClientForAuthentication(ctx, clientID)
	if err != nil {
		return domain.ClientForAuthentication{}, fmt.Errorf("postgres: machine client: %w", platformpg.MapError(err))
	}
	return domain.ClientForAuthentication{
		Subject: row.Subject, Name: row.Name, DeletedAt: platformpg.OptionalTime(row.DeletedAt),
		Organization: tenantOrganization(row.OrganizationID, row.Alias, row.Status, row.SuspensionReason),
	}, nil
}

// TouchMachineClient records a token, at most once a minute.
func (s *Store) TouchMachineClient(ctx context.Context, clientID string, at time.Time) error {
	if err := s.queries.TouchMachineClient(ctx, gen.TouchMachineClientParams{At: platformpg.Timestamptz(at), ClientID: clientID}); err != nil {
		return fmt.Errorf("postgres: touch machine client: %w", platformpg.MapError(err))
	}
	return nil
}

func apiKeyOf(r gen.APIKeyRow) domain.APIKey {
	return domain.APIKey{
		ID: r.ID, Name: r.Name,
		ExpiresAt: platformpg.OptionalTime(r.ExpiresAt), LastUsedAt: platformpg.OptionalTime(r.LastUsedAt),
		RevokedAt: platformpg.OptionalTime(r.RevokedAt), CreatedAt: platformpg.TimeOf(r.CreatedAt),
		CreatedBy: platformpg.StringOf(r.CreatedByName),
	}
}

func machineClientOf(r gen.MachineClientRow) domain.MachineClient {
	return domain.MachineClient{
		ClientID: r.ClientID, KeycloakID: r.KeycloakID.String(), Subject: r.Subject, Name: r.Name,
		SecretSetAt: platformpg.TimeOf(r.SecretSetAt), LastTokenAt: platformpg.OptionalTime(r.LastTokenAt),
		DeletedAt: platformpg.OptionalTime(r.DeletedAt), CreatedAt: platformpg.TimeOf(r.CreatedAt),
		CreatedBy: platformpg.StringOf(r.CreatedByName),
	}
}

// tenantOrganization is the organization a credential names, as far as authentication needs it.
func tenantOrganization(id pgtype.UUID, alias, status string, reason *string) domain.Organization {
	return domain.Organization{
		ID: platformpg.IDOf(id), Alias: alias, Status: domain.OrganizationStatus(status), SuspensionReason: platformpg.StringOf(reason),
	}
}
