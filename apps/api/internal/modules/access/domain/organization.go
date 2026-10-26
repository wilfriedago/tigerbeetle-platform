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
	// CreatedByName is the name of the staff member who created it, as it was then; "" for a
	// seeded organization.
	CreatedByName string
	// SuspensionReason is why staff suspended it, shown to its members.
	SuspensionReason string
}

// NewOrganization is what staff give to create an organization.
type NewOrganization struct {
	Alias      string
	Name       string
	KeycloakID string
	// CreatedBy is the staff member who created it.
	CreatedBy ledger.Principal
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
	// ListOrganizations returns up to limit organizations after the alias, in alias order.
	ListOrganizations(ctx context.Context, after string, limit int) ([]Organization, error)
	// SaveOrganization records a new organization, or returns the one with the same alias after
	// filling in its Keycloak ID when it had none, so a retried creation finishes the job.
	SaveOrganization(ctx context.Context, o NewOrganization) (Organization, error)
	// SetKeycloakID records the Keycloak ID of an organization that had none.
	SetKeycloakID(ctx context.Context, alias, keycloakID string) error
	// RenameOrganization changes the name of the organization with the alias.
	RenameOrganization(ctx context.Context, alias, name string) (Organization, error)
	// SetOrganizationStatus suspends the organization with the alias for the reason, or
	// reactivates it and drops the reason.
	SetOrganizationStatus(ctx context.Context, alias string, status OrganizationStatus, reason string) (Organization, error)
}
