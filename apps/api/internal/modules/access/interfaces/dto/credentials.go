package dto

import (
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
)

// APIKey is an organization's API key, without its secret (ADR-0029).
type APIKey struct {
	ID         string       `json:"id" doc:"The key ID, the middle of tbp_<id>_<secret>. The principal of the key is tbp:api-key/<id>."`
	Name       string       `json:"name"`
	Status     string       `json:"status" enum:"active,expired,revoked"`
	ExpiresAt  *time.Time   `json:"expiresAt,omitempty"`
	LastUsedAt *time.Time   `json:"lastUsedAt,omitempty" doc:"Updated at most once a minute."`
	RevokedAt  *time.Time   `json:"revokedAt,omitempty"`
	CreatedAt  time.Time    `json:"createdAt"`
	CreatedBy  *string      `json:"createdBy,omitempty"`
	Ledgers    []LedgerRole `json:"ledgers" doc:"The ledger roles the key acts with."`
}

// APIKeyFrom renders a key.
func APIKeyFrom(v application.APIKeyView) APIKey {
	return APIKey{
		ID: v.ID, Name: v.Name, Status: string(v.Status), ExpiresAt: v.ExpiresAt, LastUsedAt: v.LastUsedAt,
		RevokedAt: v.RevokedAt, CreatedAt: v.CreatedAt, CreatedBy: optional(v.CreatedBy), Ledgers: ledgerRolesOf(v.Ledgers),
	}
}

// APIKeyList is every key of the organization.
type APIKeyList struct {
	Data []APIKey `json:"data"`
}

// CreateAPIKeyRequest creates a key.
type CreateAPIKeyRequest struct {
	Name      string           `json:"name" pattern:"^[a-z0-9][a-z0-9-]{0,62}$" doc:"Shown in grants, write requests and logs."`
	ExpiresAt *time.Time       `json:"expiresAt,omitempty" doc:"Absent for a key that does not expire."`
	Ledgers   []LedgerRoleSpec `json:"ledgers" minItems:"1" maxItems:"100" doc:"The ledger roles the key acts with; at least one."`
}

// CreatedAPIKey is a new key and its secret, shown once.
type CreatedAPIKey struct {
	Key    APIKey `json:"key"`
	Secret string `json:"secret" doc:"The whole key, tbp_<id>_<secret>. It is not stored and cannot be shown again."`
}

// MachineClient is an organization's OAuth client in Keycloak (ADR-0029).
type MachineClient struct {
	ClientID    string       `json:"clientId" doc:"Keycloak's client ID: <organization>-<name>."`
	Name        string       `json:"name"`
	Subject     string       `json:"subject" doc:"The client's service account: the sub of its tokens."`
	Status      string       `json:"status" enum:"active,deleted"`
	SecretSetAt time.Time    `json:"secretSetAt"`
	LastTokenAt *time.Time   `json:"lastTokenAt,omitempty" doc:"When the API last saw one of its tokens; updated at most once a minute."`
	DeletedAt   *time.Time   `json:"deletedAt,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	CreatedBy   *string      `json:"createdBy,omitempty"`
	Ledgers     []LedgerRole `json:"ledgers" doc:"The ledger roles its tokens act with."`
}

// MachineClientFrom renders a machine client.
func MachineClientFrom(v application.MachineClientView) MachineClient {
	return MachineClient{
		ClientID: v.ClientID, Name: v.Name, Subject: v.Subject, Status: string(v.Status), SecretSetAt: v.SecretSetAt,
		LastTokenAt: v.LastTokenAt, DeletedAt: v.DeletedAt, CreatedAt: v.CreatedAt, CreatedBy: optional(v.CreatedBy),
		Ledgers: ledgerRolesOf(v.Ledgers),
	}
}

// MachineClientList is every machine client of the organization.
type MachineClientList struct {
	Data []MachineClient `json:"data"`
}

// CreateMachineClientRequest creates a machine client.
type CreateMachineClientRequest struct {
	Name    string           `json:"name" pattern:"^[a-z0-9][a-z0-9-]{0,62}$" doc:"The client ID becomes <organization>-<name>."`
	Ledgers []LedgerRoleSpec `json:"ledgers" minItems:"1" maxItems:"100" doc:"The ledger roles its tokens act with; at least one."`
}

// CreatedMachineClient is a new client and its secret, shown once.
type CreatedMachineClient struct {
	Client   MachineClient `json:"client"`
	Secret   string        `json:"secret" doc:"The client secret. Only a Keycloak realm administrator can see it again; regenerate it if it is lost."`
	TokenURL string        `json:"tokenUrl" doc:"Where the client trades its secret for a token, with the client credentials grant."`
}

// ClientSecret is a machine client's new secret, shown once.
type ClientSecret struct {
	Secret string `json:"secret"`
}

// LedgerGrantsOf turns requested ledger roles into the use cases' input.
func LedgerGrantsOf(specs []LedgerRoleSpec) []application.LedgerGrant {
	out := make([]application.LedgerGrant, len(specs))
	for i, s := range specs {
		out[i] = application.LedgerGrant{Ledger: s.Ledger, Role: ledger.Role(s.Role)}
	}
	return out
}

func ledgerRolesOf(roles []application.LedgerRoleView) []LedgerRole {
	out := make([]LedgerRole, len(roles))
	for i, l := range roles {
		out[i] = LedgerRole{Ledger: l.Ledger, Role: string(l.Role), GrantedAt: l.GrantedAt, GrantedBy: optional(l.GrantedBy)}
	}
	return out
}
