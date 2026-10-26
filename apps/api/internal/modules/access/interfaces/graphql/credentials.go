package graphql

import (
	"context"
	"errors"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/graphqlschema"
)

// registerCredentials adds the API keys and machine clients (ADR-0029).
func registerCredentials(s *graphqlschema.Schema) {
	graphqlschema.Describe[dto.APIKey](s, "An API key of the organization, without its secret.")
	graphqlschema.Describe[dto.CreatedAPIKey](s, "A new API key and its secret, shown once.")
	graphqlschema.Describe[dto.MachineClient](s, "An OAuth client of the organization in Keycloak.")
	graphqlschema.Describe[dto.CreatedMachineClient](s, "A new machine client and its secret, shown once.")
	graphqlschema.Describe[dto.ClientSecret](s, "A machine client's new secret, shown once.")

	graphqlschema.Query[graphqlschema.NoArgs, []dto.APIKey](s, "apiKeys", "The organization's API keys, active ones first. Needs org.credentials.")
	graphqlschema.Query[APIKeyArgs, *dto.APIKey](s, "apiKey", "One API key, or null. Needs org.credentials.")
	graphqlschema.Query[graphqlschema.NoArgs, []dto.MachineClient](s, "machineClients", "The organization's machine clients, active ones first. Needs org.credentials.")
	graphqlschema.Query[MachineClientArgs, *dto.MachineClient](s, "machineClient", "One machine client, or null. Needs org.credentials.")

	graphqlschema.Mutation[CreateAPIKeyArgs, dto.CreatedAPIKey](s, "createApiKey", "Creates an API key with its ledger roles and answers its secret, shown once.")
	graphqlschema.Mutation[APIKeyInputArgs, bool](s, "revokeApiKey", "Revokes an API key from its next request.")
	graphqlschema.Mutation[CreateMachineClientArgs, dto.CreatedMachineClient](s, "createMachineClient",
		"Creates a confidential client in Keycloak whose tokens act for the organization, and answers its secret, shown once.")
	graphqlschema.Mutation[MachineClientInputArgs, dto.ClientSecret](s, "regenerateMachineClientSecret", "Replaces a machine client's secret; the old one stops at once.")
	graphqlschema.Mutation[MachineClientInputArgs, bool](s, "deleteMachineClient", "Deletes a machine client in Keycloak.")
	graphqlschema.Mutation[SetCredentialRoleArgs, bool](s, "setCredentialRole", "Gives an API key or a machine client a role on one ledger.")
	graphqlschema.Mutation[RevokeCredentialRoleArgs, bool](s, "revokeCredentialRole", "Removes an API key's or a machine client's role on one ledger.")
}

// APIKeyArgs names a key.
type APIKeyArgs struct {
	ID string `json:"id"`
}

// MachineClientArgs names a machine client.
type MachineClientArgs struct {
	ClientID string `json:"clientId"`
}

// CreateAPIKeyInput is the input of createApiKey.
type CreateAPIKeyInput struct {
	Name      string            `json:"name" doc:"Lowercase letters, digits and dashes."`
	ExpiresAt *time.Time        `json:"expiresAt" doc:"Absent for a key that does not expire."`
	Ledgers   []LedgerRoleInput `json:"ledgers" doc:"The ledger roles the key acts with; at least one."`
}

// CreateAPIKeyArgs wraps the input.
type CreateAPIKeyArgs struct {
	Input CreateAPIKeyInput `json:"input"`
}

// APIKeyInput names a key.
type APIKeyInput struct {
	ID string `json:"id"`
}

// APIKeyInputArgs wraps the input.
type APIKeyInputArgs struct {
	Input APIKeyInput `json:"input"`
}

// CreateMachineClientInput is the input of createMachineClient.
type CreateMachineClientInput struct {
	Name    string            `json:"name" doc:"The client ID becomes <organization>-<name>."`
	Ledgers []LedgerRoleInput `json:"ledgers" doc:"The ledger roles its tokens act with; at least one."`
}

// CreateMachineClientArgs wraps the input.
type CreateMachineClientArgs struct {
	Input CreateMachineClientInput `json:"input"`
}

// MachineClientInput names a machine client.
type MachineClientInput struct {
	ClientID string `json:"clientId"`
}

// MachineClientInputArgs wraps the input.
type MachineClientInputArgs struct {
	Input MachineClientInput `json:"input"`
}

// SetCredentialRoleInput is the input of setCredentialRole.
type SetCredentialRoleInput struct {
	Kind   string `json:"kind" doc:"api-key or machine-client."`
	ID     string `json:"id" doc:"The key ID or the client ID."`
	Ledger string `json:"ledger"`
	Role   string `json:"role" doc:"viewer, operator or admin."`
}

// SetCredentialRoleArgs wraps the input.
type SetCredentialRoleArgs struct {
	Input SetCredentialRoleInput `json:"input"`
}

// RevokeCredentialRoleInput is the input of revokeCredentialRole.
type RevokeCredentialRoleInput struct {
	Kind   string `json:"kind" doc:"api-key or machine-client."`
	ID     string `json:"id"`
	Ledger string `json:"ledger"`
}

// RevokeCredentialRoleArgs wraps the input.
type RevokeCredentialRoleArgs struct {
	Input RevokeCredentialRoleInput `json:"input"`
}

func ledgerGrants(in []LedgerRoleInput) []application.LedgerGrant {
	out := make([]application.LedgerGrant, len(in))
	for i, l := range in {
		out[i] = application.LedgerGrant{Ledger: l.Ledger, Role: ledger.Role(l.Role)}
	}
	return out
}

// APIKeys resolves Query.apiKeys.
func (q *Queries) APIKeys(ctx context.Context) ([]dto.APIKey, error) {
	views, err := q.credentials.APIKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.APIKey, len(views))
	for i := range views {
		out[i] = dto.APIKeyFrom(views[i])
	}
	return out, nil
}

// APIKey resolves Query.apiKey: null when there is none.
func (q *Queries) APIKey(ctx context.Context, id string) (*dto.APIKey, error) {
	v, err := q.credentials.APIKey(ctx, id)
	if errors.Is(err, ledger.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	k := dto.APIKeyFrom(v)
	return &k, nil
}

// MachineClients resolves Query.machineClients.
func (q *Queries) MachineClients(ctx context.Context) ([]dto.MachineClient, error) {
	views, err := q.credentials.MachineClients(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.MachineClient, len(views))
	for i := range views {
		out[i] = dto.MachineClientFrom(views[i])
	}
	return out, nil
}

// MachineClient resolves Query.machineClient: null when there is none.
func (q *Queries) MachineClient(ctx context.Context, clientID string) (*dto.MachineClient, error) {
	v, err := q.credentials.MachineClient(ctx, clientID)
	if errors.Is(err, ledger.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := dto.MachineClientFrom(v)
	return &c, nil
}

// CreateAPIKey resolves Mutation.createApiKey.
func (m *Mutations) CreateAPIKey(ctx context.Context, input CreateAPIKeyInput) (*dto.CreatedAPIKey, error) {
	created, err := m.credentials.CreateAPIKey(ctx, application.NewAPIKey{Name: input.Name, ExpiresAt: input.ExpiresAt, Ledgers: ledgerGrants(input.Ledgers)})
	if err != nil {
		return nil, err
	}
	return &dto.CreatedAPIKey{Key: dto.APIKeyFrom(created.Key), Secret: created.Secret}, nil
}

// RevokeAPIKey resolves Mutation.revokeApiKey.
func (m *Mutations) RevokeAPIKey(ctx context.Context, input APIKeyInput) (bool, error) {
	return done(m.credentials.RevokeAPIKey(ctx, input.ID))
}

// CreateMachineClient resolves Mutation.createMachineClient.
func (m *Mutations) CreateMachineClient(ctx context.Context, input CreateMachineClientInput) (*dto.CreatedMachineClient, error) {
	created, err := m.credentials.CreateMachineClient(ctx, application.NewMachineClient{Name: input.Name, Ledgers: ledgerGrants(input.Ledgers)})
	if err != nil {
		return nil, err
	}
	return &dto.CreatedMachineClient{Client: dto.MachineClientFrom(created.Client), Secret: created.Secret, TokenURL: created.TokenURL}, nil
}

// RegenerateMachineClientSecret resolves Mutation.regenerateMachineClientSecret.
func (m *Mutations) RegenerateMachineClientSecret(ctx context.Context, input MachineClientInput) (*dto.ClientSecret, error) {
	secret, err := m.credentials.RegenerateSecret(ctx, input.ClientID)
	if err != nil {
		return nil, err
	}
	return &dto.ClientSecret{Secret: secret}, nil
}

// DeleteMachineClient resolves Mutation.deleteMachineClient.
func (m *Mutations) DeleteMachineClient(ctx context.Context, input MachineClientInput) (bool, error) {
	return done(m.credentials.DeleteMachineClient(ctx, input.ClientID))
}

// SetCredentialRole resolves Mutation.setCredentialRole.
func (m *Mutations) SetCredentialRole(ctx context.Context, input SetCredentialRoleInput) (bool, error) {
	return done(m.credentials.SetCredentialRole(ctx, application.CredentialKind(input.Kind), input.ID, input.Ledger, ledger.Role(input.Role)))
}

// RevokeCredentialRole resolves Mutation.revokeCredentialRole.
func (m *Mutations) RevokeCredentialRole(ctx context.Context, input RevokeCredentialRoleInput) (bool, error) {
	return done(m.credentials.RevokeCredentialRole(ctx, application.CredentialKind(input.Kind), input.ID, input.Ledger))
}
