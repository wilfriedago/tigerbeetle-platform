// Package domain is the domain layer of the access module (ADR-0027, ADR-0028): organizations as
// the platform knows them, and the ports its infrastructure implements. It imports only the
// shared kernel.
package domain

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// OrganizationStatus is the status the platform enforces on an organization.
type OrganizationStatus string

// The statuses.
const (
	OrganizationActive OrganizationStatus = "active"
	// OrganizationSuspended answers 403 to every call made in the organization's name.
	OrganizationSuspended OrganizationStatus = "suspended"
)

// Organization is the platform's copy of a Keycloak organization (ADR-0027): Keycloak owns its
// name, members and invitations; the platform's tables point at ID.
type Organization struct {
	ID ledger.ID
	// Alias is unique and immutable in Keycloak: the link between the two, and the value of the
	// X-Organization header.
	Alias  string
	Name   string
	Status OrganizationStatus
	// KeycloakID is Keycloak's identifier of the organization, once known.
	KeycloakID string
	CreatedAt  time.Time
}

// Tenant returns the organization as the tenant of a call.
func (o Organization) Tenant() ledger.Tenant {
	return ledger.Tenant{ID: o.ID, Alias: o.Alias}
}

// OrganizationRepository stores the platform's copies of organizations.
type OrganizationRepository interface {
	// OrganizationByAlias returns ledger.ErrNotFound for an unknown alias.
	OrganizationByAlias(ctx context.Context, alias string) (Organization, error)
	// OrganizationsByAlias returns the known organizations among the aliases, in name order.
	OrganizationsByAlias(ctx context.Context, aliases []string) ([]Organization, error)
}
