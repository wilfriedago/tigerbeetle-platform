package domain

import (
	"context"
	"time"
)

// Person is a person in the identity provider's directory.
type Person struct {
	// Subject is the identity provider's ID of the person: the subject of their tokens, which
	// grants name.
	Subject   string
	Email     string
	FirstName string
	LastName  string
	// SetupPending is true while an account the platform created waits for its owner to verify
	// the email and set a password (ADR-0027).
	SetupPending bool
}

// Name is the person's full name, or their email when the directory has no name.
func (p Person) Name() string {
	switch {
	case p.FirstName != "" && p.LastName != "":
		return p.FirstName + " " + p.LastName
	case p.FirstName != "":
		return p.FirstName
	case p.LastName != "":
		return p.LastName
	default:
		return p.Email
	}
}

// Invitation is an invitation the identity provider sent to a person with an account, not yet
// accepted.
type Invitation struct {
	ID        string
	Email     string
	SentAt    time.Time
	ExpiresAt time.Time
}

// Directory is the identity provider's admin API: organizations, people, memberships and
// invitations (ADR-0027). Keycloak implements it in infrastructure/keycloak. Organizations are
// named by the provider's ID of the organization, Organization.KeycloakID.
//
// Errors: ledger.ErrNotFound for an unknown organization, person, membership or invitation,
// ledger.ErrAlreadyExists for a duplicate, ledger.ErrUnavailable when the provider does not answer.
type Directory interface {
	// Issuer is the issuer of the people's tokens; grants pair it with a Subject.
	Issuer() string

	// OrganizationID returns the provider's ID of the organization with the alias.
	OrganizationID(ctx context.Context, alias string) (string, error)
	// CreateOrganization creates an organization and returns its ID.
	CreateOrganization(ctx context.Context, alias, name string) (string, error)
	// RenameOrganization changes the name; the alias never changes.
	RenameOrganization(ctx context.Context, orgID, alias, name string) error

	// PersonByEmail finds the person with the email.
	PersonByEmail(ctx context.Context, email string) (Person, error)
	// CreatePerson creates an account with the email as username, its email not yet verified and
	// the actions that verify it and set a password required.
	CreatePerson(ctx context.Context, email, firstName, lastName string) (Person, error)
	// SendAccountSetup emails the person a link to their required actions.
	SendAccountSetup(ctx context.Context, subject string) error

	// Members lists the members of the organization.
	Members(ctx context.Context, orgID string) ([]Person, error)
	// Member returns the person when they are a member of the organization.
	Member(ctx context.Context, orgID, subject string) (Person, error)
	// AddMember makes the person an unmanaged member, which removing never deletes.
	AddMember(ctx context.Context, orgID, subject string) error
	// RemoveMember ends the membership; the account stays.
	RemoveMember(ctx context.Context, orgID, subject string) error

	// Invite emails a person with an account an invitation; following it makes them a member.
	Invite(ctx context.Context, orgID, subject string) error
	// Invitations lists the invitations not yet accepted.
	Invitations(ctx context.Context, orgID string) ([]Invitation, error)
	// ResendInvitation sends the invitation again, with a new expiry.
	ResendInvitation(ctx context.Context, orgID, invitationID string) error
	// DeleteInvitation withdraws the invitation.
	DeleteInvitation(ctx context.Context, orgID, invitationID string) error
}
