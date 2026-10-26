package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// Invite is what an organization admin gives to invite a person (ADR-0027).
type Invite struct {
	Email     string
	FirstName string
	LastName  string
	// Role is the organization role the person gets; member when empty.
	Role ledger.Role
}

// MemberStatus says where a person is on the way into the organization.
type MemberStatus string

// The statuses of a member.
const (
	// MemberActive is a member who can sign in.
	MemberActive MemberStatus = "active"
	// MemberSetupPending is a member whose account the platform created and who has not yet
	// verified the email and set a password.
	MemberSetupPending MemberStatus = "setup-pending"
	// MemberInvited is a person with an account who has not yet accepted the invitation.
	MemberInvited MemberStatus = "invited"
)

// MemberView is a person in the organization with the roles the platform gives them.
type MemberView struct {
	Subject string
	Email   string
	Name    string
	Status  MemberStatus
	// Role is the organization role, "" when the person holds none.
	Role ledger.Role
	// Ledgers maps ledger names to the person's role on them.
	Ledgers map[string]ledger.Role
	// Invitation is set while Status is MemberInvited.
	Invitation *InvitationView
}

// InvitationView is a pending invitation.
type InvitationView struct {
	ID        string
	SentAt    time.Time
	ExpiresAt time.Time
	Expired   bool
}

// members are the use cases of an organization's people. Membership is Keycloak's; roles are the
// platform's grants (ADR-0027, ADR-0028). Every call acts in the organization of its tenant.
type members struct {
	authz  authorizer
	orgs   domain.OrganizationRepository
	grants domain.GrantRepository
	dir    domain.Directory
	clock  ledger.Clock
}

// List returns the members and the invited people, in name order. It needs org.members.
func (m *members) List(ctx context.Context) ([]MemberView, error) {
	org, err := m.current(ctx, ledger.PermOrgMembers, "")
	if err != nil {
		return nil, err
	}
	keycloakID, err := m.keycloakID(ctx, org)
	if err != nil {
		return nil, err
	}
	people, err := m.dir.Members(ctx, keycloakID)
	if err != nil {
		return nil, fmt.Errorf("access: members: %w", err)
	}
	invitations, err := m.dir.Invitations(ctx, keycloakID)
	if err != nil {
		return nil, fmt.Errorf("access: invitations: %w", err)
	}
	grants, err := m.grants.GrantsIn(ctx)
	if err != nil {
		return nil, fmt.Errorf("access: grants: %w", err)
	}
	bySubject := map[string][]domain.Grant{}
	for _, g := range grants {
		if g.Issuer == m.dir.Issuer() {
			bySubject[g.Subject] = append(bySubject[g.Subject], g.Grant)
		}
	}

	out := make([]MemberView, 0, len(people)+len(invitations))
	seen := map[string]bool{}
	for _, p := range people {
		status := MemberActive
		if p.SetupPending {
			status = MemberSetupPending
		}
		out = append(out, memberView(p, status, bySubject[p.Subject]))
		seen[p.Subject] = true
	}
	now := m.clock.Now()
	for _, inv := range invitations {
		// The invitation names an email; the person behind it holds the grants. An account deleted
		// since the invitation was sent leaves nobody to show.
		p, err := m.dir.PersonByEmail(ctx, inv.Email)
		if errors.Is(err, ledger.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("access: invited person: %w", err)
		}
		if seen[p.Subject] {
			continue
		}
		seen[p.Subject] = true
		v := memberView(p, MemberInvited, bySubject[p.Subject])
		v.Invitation = &InvitationView{ID: inv.ID, SentAt: inv.SentAt, ExpiresAt: inv.ExpiresAt, Expired: !now.Before(inv.ExpiresAt)}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// Invite brings a person into the organization with an organization role. It needs org.members,
// and inviting an owner needs the caller to be an owner.
func (m *members) Invite(ctx context.Context, in Invite) (MemberView, error) {
	org, err := m.current(ctx, ledger.PermOrgMembers, "")
	if err != nil {
		return MemberView{}, err
	}
	in, err = validateInvite(in)
	if err != nil {
		return MemberView{}, err
	}
	if in.Role == ledger.RoleOwner {
		if err := m.requireOwner(ctx); err != nil {
			return MemberView{}, err
		}
	}
	p, _ := ledger.PrincipalFrom(ctx)
	return m.invite(ctx, org, in, p)
}

// invite does the work of Invite without checking the caller, for staff creating an organization
// too. The roles go to the person's subject at once; they do nothing until the person is a member
// (ADR-0027). Each step tolerates an earlier attempt, so a retry finishes the job:
//
//   - no account: create it, add it as a member, email the account setup;
//   - an account the platform created and not yet set up: email the setup again;
//   - an account not in the organization: Keycloak's invitation;
//   - a member already holding an organization role: ErrAlreadyExists.
func (m *members) invite(ctx context.Context, org domain.Organization, in Invite, by ledger.Principal) (MemberView, error) {
	keycloakID, err := m.keycloakID(ctx, org)
	if err != nil {
		return MemberView{}, err
	}
	person, err := m.dir.PersonByEmail(ctx, in.Email)
	created := false
	if errors.Is(err, ledger.ErrNotFound) {
		person, err = m.dir.CreatePerson(ctx, in.Email, in.FirstName, in.LastName)
		created = true
	}
	if err != nil {
		return MemberView{}, fmt.Errorf("access: person: %w", err)
	}
	member := !created
	if member {
		if _, err := m.dir.Member(ctx, keycloakID, person.Subject); errors.Is(err, ledger.ErrNotFound) {
			member = false
		} else if err != nil {
			return MemberView{}, fmt.Errorf("access: membership: %w", err)
		}
	}
	holder := domain.Holder{Issuer: m.dir.Issuer(), Subject: person.Subject}
	if member && !person.SetupPending {
		role, err := m.orgRoleOf(ctx, holder)
		if err != nil {
			return MemberView{}, err
		}
		if role != "" {
			return MemberView{}, ledger.NewProblem(ledger.ErrAlreadyExists, "already-exists", in.Email+" is already a member")
		}
	}
	if err := m.grants.PutGrant(ctx, domain.HeldGrant{Holder: holder, Grant: domain.Grant{Role: in.Role}}, by); err != nil {
		return MemberView{}, fmt.Errorf("access: grant: %w", err)
	}

	status := MemberActive
	switch {
	case created || person.SetupPending:
		if !member {
			if err := m.dir.AddMember(ctx, keycloakID, person.Subject); err != nil && !errors.Is(err, ledger.ErrAlreadyExists) {
				return MemberView{}, fmt.Errorf("access: add member: %w", err)
			}
		}
		if err := m.dir.SendAccountSetup(ctx, person.Subject); err != nil {
			return MemberView{}, fmt.Errorf("access: account setup email: %w", err)
		}
		status = MemberSetupPending
	case !member:
		if err := m.dir.Invite(ctx, keycloakID, person.Subject); err != nil {
			return MemberView{}, fmt.Errorf("access: invitation: %w", err)
		}
		status = MemberInvited
	}
	return memberView(person, status, []domain.Grant{{Role: in.Role}}), nil
}

// SetRole changes a person's organization role. It needs org.members; granting or taking away
// owner needs the caller to be an owner, and the organization keeps at least one owner.
func (m *members) SetRole(ctx context.Context, subject string, role ledger.Role) error {
	if _, err := m.current(ctx, ledger.PermOrgMembers, ""); err != nil {
		return err
	}
	if !isOneOf(role, ledger.OrganizationRoles) {
		return invalid("role must be member, admin or owner")
	}
	holder := domain.Holder{Issuer: m.dir.Issuer(), Subject: subject}
	current, err := m.knownRole(ctx, holder)
	if err != nil {
		return err
	}
	if current == ledger.RoleOwner || role == ledger.RoleOwner {
		if err := m.requireOwner(ctx); err != nil {
			return err
		}
	}
	if current == ledger.RoleOwner && role != ledger.RoleOwner {
		if err := m.keepAnOwner(ctx); err != nil {
			return err
		}
	}
	p, _ := ledger.PrincipalFrom(ctx)
	if err := m.grants.PutGrant(ctx, domain.HeldGrant{Holder: holder, Grant: domain.Grant{Role: role}}, p); err != nil {
		return fmt.Errorf("access: grant: %w", err)
	}
	return nil
}

// Remove ends a person's membership and removes every role they hold in the organization; their
// account and other memberships stay. Removing an owner needs the caller to be an owner, and the
// last owner stays. A person still invited is removed by deleting the invitation.
func (m *members) Remove(ctx context.Context, subject string) error {
	org, err := m.current(ctx, ledger.PermOrgMembers, "")
	if err != nil {
		return err
	}
	keycloakID, err := m.keycloakID(ctx, org)
	if err != nil {
		return err
	}
	holder := domain.Holder{Issuer: m.dir.Issuer(), Subject: subject}
	role, err := m.orgRoleOf(ctx, holder)
	if err != nil {
		return err
	}
	if role == ledger.RoleOwner {
		if err := m.requireOwner(ctx); err != nil {
			return err
		}
		if err := m.keepAnOwner(ctx); err != nil {
			return err
		}
	}
	if err := m.dir.RemoveMember(ctx, keycloakID, subject); err != nil {
		return fmt.Errorf("access: remove member: %w", err)
	}
	if err := m.grants.RevokeGrants(ctx, holder); err != nil {
		return fmt.Errorf("access: revoke grants: %w", err)
	}
	return nil
}

// SetLedgerRole gives a member a role on one ledger. It needs ledger.members on that ledger.
func (m *members) SetLedgerRole(ctx context.Context, subject, ledgerName string, role ledger.Role) error {
	if _, err := m.current(ctx, ledger.PermLedgerMembers, ledgerName); err != nil {
		return err
	}
	if !isOneOf(role, ledger.LedgerRoles) {
		return invalid("role must be viewer, operator or admin")
	}
	holder := domain.Holder{Issuer: m.dir.Issuer(), Subject: subject}
	if _, err := m.knownRole(ctx, holder); err != nil {
		return err
	}
	p, _ := ledger.PrincipalFrom(ctx)
	if err := m.grants.PutGrant(ctx, domain.HeldGrant{Holder: holder, Grant: domain.Grant{Ledger: ledgerName, Role: role}}, p); err != nil {
		return fmt.Errorf("access: grant: %w", err)
	}
	return nil
}

// RevokeLedgerRole removes a person's role on one ledger. It needs ledger.members on that ledger.
func (m *members) RevokeLedgerRole(ctx context.Context, subject, ledgerName string) error {
	if _, err := m.current(ctx, ledger.PermLedgerMembers, ledgerName); err != nil {
		return err
	}
	if err := m.grants.RevokeGrant(ctx, domain.Holder{Issuer: m.dir.Issuer(), Subject: subject}, ledgerName); err != nil {
		return fmt.Errorf("access: revoke: %w", err)
	}
	return nil
}

// ResendInvitation sends an invitation again with a new expiry. It needs org.members.
func (m *members) ResendInvitation(ctx context.Context, id string) error {
	org, err := m.current(ctx, ledger.PermOrgMembers, "")
	if err != nil {
		return err
	}
	keycloakID, err := m.keycloakID(ctx, org)
	if err != nil {
		return err
	}
	if err := m.dir.ResendInvitation(ctx, keycloakID, id); err != nil {
		return fmt.Errorf("access: resend invitation: %w", err)
	}
	return nil
}

// DeleteInvitation withdraws an invitation and the roles waiting for the person, unless they
// became a member meanwhile. It needs org.members; withdrawing an owner's needs an owner.
func (m *members) DeleteInvitation(ctx context.Context, id string) error {
	org, err := m.current(ctx, ledger.PermOrgMembers, "")
	if err != nil {
		return err
	}
	keycloakID, err := m.keycloakID(ctx, org)
	if err != nil {
		return err
	}
	invitations, err := m.dir.Invitations(ctx, keycloakID)
	if err != nil {
		return fmt.Errorf("access: invitations: %w", err)
	}
	var email string
	for _, inv := range invitations {
		if inv.ID == id {
			email = inv.Email
		}
	}
	if email == "" {
		return fmt.Errorf("access: invitation %q: %w", id, ledger.ErrNotFound)
	}
	person, err := m.dir.PersonByEmail(ctx, email)
	switch {
	case errors.Is(err, ledger.ErrNotFound):
	case err != nil:
		return fmt.Errorf("access: invited person: %w", err)
	default:
		if err := m.withdrawRoles(ctx, keycloakID, person); err != nil {
			return err
		}
	}
	if err := m.dir.DeleteInvitation(ctx, keycloakID, id); err != nil {
		return fmt.Errorf("access: delete invitation: %w", err)
	}
	return nil
}

// withdrawRoles revokes the roles of an invited person who is not a member.
func (m *members) withdrawRoles(ctx context.Context, keycloakID string, person domain.Person) error {
	if _, err := m.dir.Member(ctx, keycloakID, person.Subject); err == nil {
		return nil
	} else if !errors.Is(err, ledger.ErrNotFound) {
		return fmt.Errorf("access: membership: %w", err)
	}
	holder := domain.Holder{Issuer: m.dir.Issuer(), Subject: person.Subject}
	role, err := m.orgRoleOf(ctx, holder)
	if err != nil {
		return err
	}
	if role == ledger.RoleOwner {
		if err := m.requireOwner(ctx); err != nil {
			return err
		}
	}
	if err := m.grants.RevokeGrants(ctx, holder); err != nil {
		return fmt.Errorf("access: revoke grants: %w", err)
	}
	return nil
}

// current checks the permission and returns the organization of the call.
func (m *members) current(ctx context.Context, perm ledger.Permission, ledgerName string) (domain.Organization, error) {
	if err := m.authz.Require(ctx, perm, ledgerName); err != nil {
		return domain.Organization{}, err
	}
	return m.organization(ctx)
}

// organization returns the organization of the call.
func (m *members) organization(ctx context.Context) (domain.Organization, error) {
	t, err := ledger.RequireTenant(ctx)
	if err != nil {
		return domain.Organization{}, err
	}
	org, err := m.orgs.OrganizationByAlias(ctx, t.Alias)
	if err != nil {
		return domain.Organization{}, fmt.Errorf("access: organization %q: %w", t.Alias, err)
	}
	return org, nil
}

// keycloakID returns Keycloak's ID of the organization, looking it up and recording it for an
// organization seeded without one.
func (m *members) keycloakID(ctx context.Context, org domain.Organization) (string, error) {
	if org.KeycloakID != "" {
		return org.KeycloakID, nil
	}
	id, err := m.dir.OrganizationID(ctx, org.Alias)
	if err != nil {
		return "", fmt.Errorf("access: organization %q in the directory: %w", org.Alias, err)
	}
	if err := m.orgs.SetKeycloakID(ctx, org.Alias, id); err != nil {
		return "", fmt.Errorf("access: record the directory ID of %q: %w", org.Alias, err)
	}
	return id, nil
}

// orgRoleOf returns the organization role a holder has, "" for none.
func (m *members) orgRoleOf(ctx context.Context, h domain.Holder) (ledger.Role, error) {
	grants, err := m.grants.GrantsOf(ctx, h.Issuer, h.Subject)
	if err != nil {
		return "", fmt.Errorf("access: grants: %w", err)
	}
	for _, g := range grants {
		if g.Ledger == "" {
			return g.Role, nil
		}
	}
	return "", nil
}

// knownRole returns the organization role of a person the organization knows: one holding an
// organization role (a member or an invited person), or a member of the organization in Keycloak
// without one. Anyone else is ErrNotFound, so roles never go to strangers.
func (m *members) knownRole(ctx context.Context, h domain.Holder) (ledger.Role, error) {
	role, err := m.orgRoleOf(ctx, h)
	if err != nil || role != "" {
		return role, err
	}
	org, err := m.organization(ctx)
	if err != nil {
		return "", err
	}
	keycloakID, err := m.keycloakID(ctx, org)
	if err != nil {
		return "", err
	}
	if _, err := m.dir.Member(ctx, keycloakID, h.Subject); err != nil {
		return "", fmt.Errorf("access: member %q: %w", h.Subject, err)
	}
	return "", nil
}

// requireOwner refuses a caller who is not an owner of the organization.
func (m *members) requireOwner(ctx context.Context) error {
	p, ok := ledger.PrincipalFrom(ctx)
	if !ok {
		return ledger.ErrUnauthenticated
	}
	if p.Kind == ledger.KindSystem {
		return nil
	}
	role, err := m.orgRoleOf(ctx, domain.Holder{Issuer: p.Issuer, Subject: p.Subject})
	if err != nil {
		return err
	}
	if role != ledger.RoleOwner {
		return ledger.NewProblem(ledger.ErrForbidden, "forbidden", "only an owner can grant or take away the owner role")
	}
	return nil
}

// keepAnOwner refuses to take away the last owner.
func (m *members) keepAnOwner(ctx context.Context) error {
	n, err := m.grants.CountOwners(ctx)
	if err != nil {
		return fmt.Errorf("access: owners: %w", err)
	}
	if n <= 1 {
		return ledger.NewProblem(ledger.ErrConflict, "last-owner", "an organization keeps at least one owner")
	}
	return nil
}

func memberView(p domain.Person, status MemberStatus, grants []domain.Grant) MemberView {
	v := MemberView{Subject: p.Subject, Email: p.Email, Name: p.Name(), Status: status, Ledgers: map[string]ledger.Role{}}
	for _, g := range grants {
		if g.Ledger == "" {
			v.Role = g.Role
		} else {
			v.Ledgers[g.Ledger] = g.Role
		}
	}
	return v
}
