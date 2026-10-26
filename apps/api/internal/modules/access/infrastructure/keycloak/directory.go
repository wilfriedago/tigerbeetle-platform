// Package keycloak implements the access module's Directory on the Keycloak admin client of
// internal/platform/keycloak (ADR-0027). It only translates: Keycloak's users are the domain's
// people, and a user's ID is the subject of their tokens.
package keycloak

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
	platformkc "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/keycloak"
)

// Directory is the access module's domain.Directory on Keycloak.
type Directory struct {
	admin  *platformkc.Admin
	issuer string
}

var _ domain.Directory = (*Directory)(nil)

// New adapts the admin client. issuer is the issuer of people's tokens, which grants pair with
// their subject.
func New(admin *platformkc.Admin, issuer string) *Directory {
	return &Directory{admin: admin, issuer: issuer}
}

// Issuer is the issuer of people's tokens.
func (d *Directory) Issuer() string { return d.issuer }

// OrganizationID returns Keycloak's ID of the organization with the alias.
func (d *Directory) OrganizationID(ctx context.Context, alias string) (string, error) {
	return d.admin.OrganizationID(ctx, alias)
}

// CreateOrganization creates the organization and returns its ID.
func (d *Directory) CreateOrganization(ctx context.Context, alias, name string) (string, error) {
	return d.admin.CreateOrganization(ctx, alias, name)
}

// RenameOrganization changes the name.
func (d *Directory) RenameOrganization(ctx context.Context, orgID, alias, name string) error {
	return d.admin.RenameOrganization(ctx, orgID, alias, name)
}

// PersonByEmail finds the person with the email.
func (d *Directory) PersonByEmail(ctx context.Context, email string) (domain.Person, error) {
	u, err := d.admin.UserByEmail(ctx, email)
	return person(u), err
}

// CreatePerson creates an account that waits for its setup.
func (d *Directory) CreatePerson(ctx context.Context, email, firstName, lastName string) (domain.Person, error) {
	u, err := d.admin.CreateUser(ctx, email, firstName, lastName)
	return person(u), err
}

// SendAccountSetup emails the link to the account setup.
func (d *Directory) SendAccountSetup(ctx context.Context, subject string) error {
	return d.admin.SendAccountSetup(ctx, subject)
}

// Members lists the members of the organization.
func (d *Directory) Members(ctx context.Context, orgID string) ([]domain.Person, error) {
	users, err := d.admin.Members(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Person, len(users))
	for i, u := range users {
		out[i] = person(u)
	}
	return out, nil
}

// Member returns the person when they are a member.
func (d *Directory) Member(ctx context.Context, orgID, subject string) (domain.Person, error) {
	u, err := d.admin.Member(ctx, orgID, subject)
	return person(u), err
}

// AddMember makes the person an unmanaged member.
func (d *Directory) AddMember(ctx context.Context, orgID, subject string) error {
	return d.admin.AddMember(ctx, orgID, subject)
}

// RemoveMember ends the membership.
func (d *Directory) RemoveMember(ctx context.Context, orgID, subject string) error {
	return d.admin.RemoveMember(ctx, orgID, subject)
}

// Invite emails Keycloak's invitation.
func (d *Directory) Invite(ctx context.Context, orgID, subject string) error {
	return d.admin.Invite(ctx, orgID, subject)
}

// Invitations lists the invitations not yet accepted.
func (d *Directory) Invitations(ctx context.Context, orgID string) ([]domain.Invitation, error) {
	invs, err := d.admin.Invitations(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Invitation, len(invs))
	for i, inv := range invs {
		out[i] = domain.Invitation{
			ID: inv.ID, Email: inv.Email,
			SentAt: time.Unix(inv.SentDate, 0).UTC(), ExpiresAt: time.Unix(inv.ExpiresAt, 0).UTC(),
		}
	}
	return out, nil
}

// ResendInvitation sends the invitation again.
func (d *Directory) ResendInvitation(ctx context.Context, orgID, invitationID string) error {
	return d.admin.ResendInvitation(ctx, orgID, invitationID)
}

// DeleteInvitation withdraws the invitation.
func (d *Directory) DeleteInvitation(ctx context.Context, orgID, invitationID string) error {
	return d.admin.DeleteInvitation(ctx, orgID, invitationID)
}

func person(u platformkc.User) domain.Person {
	return domain.Person{
		Subject: u.ID, Email: u.Email, FirstName: u.FirstName, LastName: u.LastName,
		SetupPending: u.SetupPending(),
	}
}
