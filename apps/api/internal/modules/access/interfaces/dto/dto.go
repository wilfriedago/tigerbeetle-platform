// Package dto holds the wire types of the access module that REST answers and GraphQL binds to
// (ADR-0027, ADR-0028): organizations as staff see them, and the members of an organization with
// their roles. The struct tags carry the descriptions and constraints of both specs; the domain
// checks the constraints.
package dto

import (
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// Organization is a tenant of the platform, as staff see it: who it is, its owners and counts,
// never its ledgers' contents.
type Organization struct {
	ID               string    `json:"id" doc:"The platform's ID of the organization."`
	KeycloakID       string    `json:"keycloakId,omitempty" doc:"Keycloak's ID of the organization."`
	Alias            string    `json:"alias" doc:"Unique and permanent: the value of X-Organization and the organization's URL segment."`
	Name             string    `json:"name"`
	Status           string    `json:"status" enum:"active,suspended" doc:"A suspended organization answers 403 organization-suspended to every call made in its name."`
	SuspensionReason *string   `json:"suspensionReason,omitempty" doc:"Why staff suspended it, shown to its members."`
	CreatedAt        time.Time `json:"createdAt"`
	CreatedBy        *string   `json:"createdBy,omitempty" doc:"The staff member who created it; absent for a seeded organization."`
	Owners           []Owner   `json:"owners" doc:"The owners, oldest first, and the first owner while on the way in."`
	Members          int       `json:"members" doc:"The number of members in Keycloak."`
	Ledgers          int       `json:"ledgers" doc:"The number of ledgers."`
}

// Owner is an owner of an organization.
type Owner struct {
	Subject string    `json:"subject"`
	Email   string    `json:"email"`
	Name    string    `json:"name"`
	Status  string    `json:"status" enum:"active,setup-pending,invited"`
	Since   time.Time `json:"since" doc:"When the owner role was granted."`
}

// OrganizationFrom renders an organization.
func OrganizationFrom(v application.OrganizationView) Organization {
	o := Organization{
		ID: uuidOf(v.ID), KeycloakID: v.KeycloakID, Alias: v.Alias, Name: v.Name, Status: string(v.Status),
		SuspensionReason: optional(v.SuspensionReason), CreatedAt: v.CreatedAt, CreatedBy: optional(v.CreatedBy),
		Owners: make([]Owner, len(v.Owners)), Members: v.Members, Ledgers: v.Ledgers,
	}
	for i, ow := range v.Owners {
		o.Owners[i] = Owner{Subject: ow.Subject, Email: ow.Email, Name: ow.Name, Status: string(ow.Status), Since: ow.Since}
	}
	return o
}

// uuidOf renders the platform's ID of an organization as a UUID, as Postgres stores it.
func uuidOf(id ledger.ID) string {
	h := id.String()
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// OrganizationList is a page of organizations.
type OrganizationList struct {
	Data []Organization `json:"data"`
	Next *string        `json:"next,omitempty" doc:"The cursor of the next page; absent on the last one."`
}

// NewMember names the person an invitation is for.
type NewMember struct {
	Email     string `json:"email" format:"email" maxLength:"254"`
	FirstName string `json:"firstName,omitempty" maxLength:"100" doc:"Used when the person has no account yet."`
	LastName  string `json:"lastName,omitempty" maxLength:"100" doc:"Used when the person has no account yet."`
}

// CreateOrganizationRequest creates an organization and invites its first owner.
type CreateOrganizationRequest struct {
	Alias string    `json:"alias" pattern:"^[a-z0-9][a-z0-9-]{0,62}$" doc:"Lowercase letters, digits and dashes, 1 to 63 characters. It cannot change."`
	Name  string    `json:"name" minLength:"1" maxLength:"100"`
	Owner NewMember `json:"owner" doc:"The first owner. A person without an account gets one and an email to set it up; a person with one gets an invitation."`
}

// UpdateOrganizationRequest renames, suspends or reactivates an organization. Absent fields stay.
type UpdateOrganizationRequest struct {
	Name   *string `json:"name,omitempty" minLength:"1" maxLength:"100"`
	Status *string `json:"status,omitempty" enum:"active,suspended"`
	Reason *string `json:"reason,omitempty" maxLength:"500" doc:"Why the organization is suspended, shown to its members. Used with status suspended."`
}

// InviteMemberRequest invites a person with an organization role and, optionally, ledger roles.
type InviteMemberRequest struct {
	NewMember
	Role    string           `json:"role,omitempty" enum:"member,admin,owner" default:"member" doc:"Only an owner can invite an owner."`
	Ledgers []LedgerRoleSpec `json:"ledgers,omitempty" maxItems:"100" doc:"Roles on single ledgers, granted with the organization role."`
}

// LedgerRoleSpec names a role on one ledger.
type LedgerRoleSpec struct {
	Ledger string `json:"ledger"`
	Role   string `json:"role" enum:"viewer,operator,admin"`
}

// OrganizationRoleRequest sets a person's organization role.
type OrganizationRoleRequest struct {
	Role string `json:"role" enum:"member,admin,owner" doc:"Only an owner can grant or take away owner; the last owner stays."`
}

// LedgerRoleRequest sets a person's role on one ledger.
type LedgerRoleRequest struct {
	Role string `json:"role" enum:"viewer,operator,admin"`
}

// LedgerRole is a role on one ledger, with when and by whom it was granted.
type LedgerRole struct {
	Ledger    string    `json:"ledger"`
	Role      string    `json:"role" enum:"viewer,operator,admin"`
	GrantedAt time.Time `json:"grantedAt"`
	GrantedBy *string   `json:"grantedBy,omitempty" doc:"The name of who granted it, as it was then."`
}

// Invitation is the invitation a person has not yet accepted.
type Invitation struct {
	ID        string    `json:"id"`
	SentAt    time.Time `json:"sentAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Expired   bool      `json:"expired" doc:"An expired invitation can be resent."`
}

// Member is a person of the organization with the roles the platform gives them.
type Member struct {
	Subject       string     `json:"subject" doc:"The identity provider's ID of the person: the subject of their tokens."`
	Email         string     `json:"email"`
	EmailVerified bool       `json:"emailVerified"`
	Name          string     `json:"name"`
	Status        string     `json:"status" enum:"active,setup-pending,invited" doc:"setup-pending: the platform created the account and the person has not yet set a password. invited: the person has not yet accepted the invitation."`
	Role          *string    `json:"role,omitempty" enum:"member,admin,owner" doc:"The organization role; absent when the person holds none."`
	AddedAt       *time.Time `json:"addedAt,omitempty" doc:"When the organization role was granted."`
	AddedBy       *string    `json:"addedBy,omitempty" doc:"The name of who granted the organization role, as it was then."`
	// Ledgers lists the ledger roles in ledger name order.
	Ledgers    []LedgerRole `json:"ledgers"`
	Invitation *Invitation  `json:"invitation,omitempty" doc:"Present while status is invited."`
}

// MemberFrom renders a member.
func MemberFrom(v application.MemberView) Member {
	m := Member{
		Subject: v.Subject, Email: v.Email, EmailVerified: v.EmailVerified, Name: v.Name, Status: string(v.Status),
		Role: optional(string(v.Role)), AddedBy: optional(v.AddedBy), Ledgers: make([]LedgerRole, len(v.Ledgers)),
	}
	if !v.AddedAt.IsZero() {
		m.AddedAt = &v.AddedAt
	}
	for i, l := range v.Ledgers {
		m.Ledgers[i] = LedgerRole{Ledger: l.Ledger, Role: string(l.Role), GrantedAt: l.GrantedAt, GrantedBy: optional(l.GrantedBy)}
	}
	if v.Invitation != nil {
		m.Invitation = &Invitation{ID: v.Invitation.ID, SentAt: v.Invitation.SentAt, ExpiresAt: v.Invitation.ExpiresAt, Expired: v.Invitation.Expired}
	}
	return m
}

// MemberList is every member of the organization, invited people included.
type MemberList struct {
	Data []Member `json:"data"`
}

// InviteFrom turns a request into the use case's input.
func InviteFrom(m NewMember, role string, ledgers []LedgerRoleSpec) application.Invite {
	in := application.Invite{Email: m.Email, FirstName: m.FirstName, LastName: m.LastName, Role: ledger.Role(role)}
	for _, l := range ledgers {
		in.Ledgers = append(in.Ledgers, application.LedgerGrant{Ledger: l.Ledger, Role: ledger.Role(l.Role)})
	}
	return in
}

// StatusOf turns a wire status into the domain's.
func StatusOf(s string) domain.OrganizationStatus { return domain.OrganizationStatus(s) }
