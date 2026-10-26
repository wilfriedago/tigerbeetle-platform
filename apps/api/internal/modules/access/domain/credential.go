package domain

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// APIKeyIssuer is the issuer of every API key's principal: a key is (APIKeyIssuer, key ID) in
// grants, write records and logs (ADR-0029).
const APIKeyIssuer = "tbp:api-key" //nolint:gosec // an issuer name, not a credential

// APIKey is an organization's API key. Its secret is never stored, only SecretHash.
type APIKey struct {
	// ID is the middle part of tbp_<id>_<secret>: eight letters and digits.
	ID         string
	Name       string
	SecretHash []byte
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	// CreatedBy is the name of who created it, as it was then.
	CreatedBy string
}

// KeyForAuthentication is what authentication reads of a key, with the organization it belongs to.
type KeyForAuthentication struct {
	Name         string
	SecretHash   []byte
	ExpiresAt    *time.Time
	RevokedAt    *time.Time
	Organization Organization
}

// APIKeyRepository stores API keys. Every method but the two for authentication acts in the
// organization of the call.
type APIKeyRepository interface {
	CreateAPIKey(ctx context.Context, k APIKey, by ledger.Principal) error
	APIKeys(ctx context.Context) ([]APIKey, error)
	// APIKey returns ledger.ErrNotFound for a key of another organization or none.
	APIKey(ctx context.Context, id string) (APIKey, error)
	// RevokeAPIKey returns ledger.ErrNotFound when no active key has the ID.
	RevokeAPIKey(ctx context.Context, id string, at time.Time) error
	// APIKeyForAuthentication finds a key by ID in any organization: the key names its own.
	APIKeyForAuthentication(ctx context.Context, id string) (KeyForAuthentication, error)
	// TouchAPIKey records a use, at most once a minute.
	TouchAPIKey(ctx context.Context, id string, at time.Time) error
}

// MachineClient is an organization's OAuth client in Keycloak, used with client credentials.
type MachineClient struct {
	ClientID   string
	KeycloakID string
	// Subject is the client's service account: the sub of its tokens, which grants name.
	Subject     string
	Name        string
	SecretSetAt time.Time
	LastTokenAt *time.Time
	DeletedAt   *time.Time
	CreatedAt   time.Time
	CreatedBy   string
}

// ClientForAuthentication is what authentication reads of a machine client.
type ClientForAuthentication struct {
	Subject      string
	Name         string
	DeletedAt    *time.Time
	Organization Organization
}

// MachineClientRepository stores machine clients. Every method but the two for authentication
// acts in the organization of the call.
type MachineClientRepository interface {
	CreateMachineClient(ctx context.Context, c MachineClient, by ledger.Principal) error
	MachineClients(ctx context.Context) ([]MachineClient, error)
	// MachineClient returns ledger.ErrNotFound for a client of another organization or none.
	MachineClient(ctx context.Context, clientID string) (MachineClient, error)
	// SetSecretSetAt records a new secret; ledger.ErrNotFound for no active client.
	SetSecretSetAt(ctx context.Context, clientID string, at time.Time) error
	// DeleteMachineClient marks the client deleted; ledger.ErrNotFound for no active client.
	DeleteMachineClient(ctx context.Context, clientID string, at time.Time) error
	// MachineClientForAuthentication finds a client by ID in any organization.
	MachineClientForAuthentication(ctx context.Context, clientID string) (ClientForAuthentication, error)
	// TouchMachineClient records a token, at most once a minute.
	TouchMachineClient(ctx context.Context, clientID string, at time.Time) error
}

// NewClient is a confidential client with a service account, whose tokens carry the API's
// audience.
type NewClient struct {
	ClientID string
	Name     string
}

// CreatedClient is what the identity provider answers for a new client: its IDs and secret.
type CreatedClient struct {
	KeycloakID string
	// Subject is the client's service account user.
	Subject string
	Secret  string
}

// Clients is the identity provider's admin API for machine clients (ADR-0029). Keycloak
// implements it with Directory.
type Clients interface {
	CreateClient(ctx context.Context, c NewClient) (CreatedClient, error)
	RegenerateSecret(ctx context.Context, keycloakID string) (string, error)
	DeleteClient(ctx context.Context, keycloakID string) error
	// TokenURL is where clients trade their secret for a token.
	TokenURL() string
}
