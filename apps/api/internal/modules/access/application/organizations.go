package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// OrganizationView is an organization as staff see it: who it is, its owners, and counts, never
// its ledgers' contents.
type OrganizationView struct {
	ID         ledger.ID
	KeycloakID string
	Alias      string
	Name       string
	Status     domain.OrganizationStatus
	// SuspensionReason is set while the organization is suspended.
	SuspensionReason string
	CreatedAt        time.Time
	// CreatedBy is the name of the staff member who created it; "" for a seeded organization.
	CreatedBy string
	Owners    []OwnerView
	Members   int
	Ledgers   int
}

// OwnerView is an owner of an organization, or the first owner on their way in.
type OwnerView struct {
	Subject string
	Email   string
	Name    string
	Status  MemberStatus
	Since   time.Time
}

func organizationView(o domain.Organization) OrganizationView {
	return OrganizationView{
		ID: o.ID, KeycloakID: o.KeycloakID, Alias: o.Alias, Name: o.Name, Status: o.Status,
		SuspensionReason: o.SuspensionReason, CreatedAt: o.CreatedAt, CreatedBy: o.CreatedByName,
	}
}

// CreateOrganization is what staff give to create an organization and invite its first owner.
type CreateOrganization struct {
	Alias string
	Name  string
	Owner Invite
}

// platformOrganizations are the staff's use cases (ADR-0027): every one needs
// platform.organizations, which only staff hold, and none acts inside an organization.
type platformOrganizations struct {
	authz   authorizer
	orgs    domain.OrganizationRepository
	grants  domain.GrantRepository
	members *members
	ledgers LedgerCounter
}

// describe adds the owners and the counts to organizations: one query for the owners, one for the
// ledger counts, and calls to the directory for each organization's member count and each
// owner's account.
func (u platformOrganizations) describe(ctx context.Context, orgs []domain.Organization) ([]OrganizationView, error) {
	ids := make([]ledger.ID, len(orgs))
	for i := range orgs {
		ids[i] = orgs[i].ID
	}
	owners, err := u.grants.OwnersIn(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("access: owners: %w", err)
	}
	counts := map[ledger.ID]int{}
	if u.ledgers != nil {
		if counts, err = u.ledgers.CountLedgers(ctx, ids); err != nil {
			return nil, fmt.Errorf("access: ledger counts: %w", err)
		}
	}
	dir := u.members.dir
	out := make([]OrganizationView, len(orgs))
	for i := range orgs {
		o := &orgs[i]
		v := organizationView(*o)
		v.Ledgers = counts[o.ID]
		keycloakID, err := u.members.keycloakID(ctx, *o)
		if errors.Is(err, ledger.ErrNotFound) {
			// Known to the platform but not to Keycloak, such as a row seeded by hand: the
			// counts and owners Keycloak holds stay empty rather than failing the page.
			out[i] = v
			continue
		}
		if err != nil {
			return nil, err
		}
		v.KeycloakID = keycloakID
		if v.Members, err = dir.MemberCount(ctx, keycloakID); err != nil {
			return nil, fmt.Errorf("access: members of %q: %w", o.Alias, err)
		}
		for _, owner := range owners[o.ID] {
			if owner.Issuer != dir.Issuer() {
				continue
			}
			p, err := dir.PersonBySubject(ctx, owner.Subject)
			if errors.Is(err, ledger.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("access: owner of %q: %w", o.Alias, err)
			}
			status, err := u.members.statusOf(ctx, keycloakID, p)
			if err != nil {
				return nil, err
			}
			v.Owners = append(v.Owners, OwnerView{Subject: p.Subject, Email: p.Email, Name: p.Name(), Status: status, Since: owner.Since})
		}
		out[i] = v
	}
	return out, nil
}

// describeOne is describe for one organization.
func (u platformOrganizations) describeOne(ctx context.Context, o domain.Organization) (OrganizationView, error) {
	views, err := u.describe(ctx, []domain.Organization{o})
	if err != nil {
		return OrganizationView{}, err
	}
	return views[0], nil
}

// List returns a page of every organization, in alias order.
func (u platformOrganizations) List(ctx context.Context, cursor string, limit int) ([]OrganizationView, string, error) {
	if err := u.authz.Require(ctx, ledger.PermPlatformOrganizations, ""); err != nil {
		return nil, "", err
	}
	after, err := ledger.DecodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	limit = ledger.PageSize(limit)
	rows, err := u.orgs.ListOrganizations(ctx, after, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("access: organizations: %w", err)
	}
	page, next := ledger.TrimPage(rows, limit, func(o domain.Organization) string { return o.Alias })
	out, err := u.describe(ctx, page)
	if err != nil {
		return nil, "", err
	}
	return out, next, nil
}

// Get returns one organization.
func (u platformOrganizations) Get(ctx context.Context, alias string) (OrganizationView, error) {
	if err := u.authz.Require(ctx, ledger.PermPlatformOrganizations, ""); err != nil {
		return OrganizationView{}, err
	}
	o, err := u.orgs.OrganizationByAlias(ctx, alias)
	if err != nil {
		return OrganizationView{}, fmt.Errorf("access: organization %q: %w", alias, err)
	}
	return u.describeOne(ctx, o)
}

// Create creates the organization in Keycloak, records the platform's copy and invites the first
// owner. Each step finds what an earlier, interrupted attempt left, so retrying with the same
// alias finishes the job; an organization that already has an owner is ErrAlreadyExists.
func (u platformOrganizations) Create(ctx context.Context, in CreateOrganization) (OrganizationView, error) {
	if err := u.authz.Require(ctx, ledger.PermPlatformOrganizations, ""); err != nil {
		return OrganizationView{}, err
	}
	p, _ := ledger.PrincipalFrom(ctx)
	name, err := validateOrganization(in.Alias, in.Name)
	if err != nil {
		return OrganizationView{}, err
	}
	owner, err := validateInvite(Invite{Email: in.Owner.Email, FirstName: in.Owner.FirstName, LastName: in.Owner.LastName, Role: ledger.RoleOwner})
	if err != nil {
		return OrganizationView{}, err
	}

	existing, err := u.orgs.OrganizationByAlias(ctx, in.Alias)
	switch {
	case err == nil:
		owners, err := u.grants.CountOwners(ledger.WithTenant(ctx, existing.Tenant()))
		if err != nil {
			return OrganizationView{}, fmt.Errorf("access: owners of %q: %w", in.Alias, err)
		}
		if owners > 0 {
			return OrganizationView{}, ledger.NewProblem(ledger.ErrAlreadyExists, "already-exists", "the alias "+in.Alias+" is taken")
		}
	case !errors.Is(err, ledger.ErrNotFound):
		return OrganizationView{}, fmt.Errorf("access: organization %q: %w", in.Alias, err)
	}

	keycloakID, err := u.members.dir.OrganizationID(ctx, in.Alias)
	if errors.Is(err, ledger.ErrNotFound) {
		keycloakID, err = u.members.dir.CreateOrganization(ctx, in.Alias, name)
	}
	if err != nil {
		return OrganizationView{}, fmt.Errorf("access: create %q in the directory: %w", in.Alias, err)
	}
	org, err := u.orgs.SaveOrganization(ctx, domain.NewOrganization{Alias: in.Alias, Name: name, KeycloakID: keycloakID, CreatedBy: p})
	if err != nil {
		return OrganizationView{}, fmt.Errorf("access: save %q: %w", in.Alias, err)
	}
	if _, err := u.members.invite(ledger.WithTenant(ctx, org.Tenant()), org, owner, p); err != nil {
		return OrganizationView{}, fmt.Errorf("access: invite the owner of %q: %w", in.Alias, err)
	}
	return u.describeOne(ctx, org)
}

// Rename changes the name in Keycloak and in the platform's copy; the alias never changes.
func (u platformOrganizations) Rename(ctx context.Context, alias, name string) (OrganizationView, error) {
	if err := u.authz.Require(ctx, ledger.PermPlatformOrganizations, ""); err != nil {
		return OrganizationView{}, err
	}
	name, err := validateOrganization(alias, name)
	if err != nil {
		return OrganizationView{}, err
	}
	org, err := u.orgs.OrganizationByAlias(ctx, alias)
	if err != nil {
		return OrganizationView{}, fmt.Errorf("access: organization %q: %w", alias, err)
	}
	keycloakID, err := u.members.keycloakID(ctx, org)
	if err != nil {
		return OrganizationView{}, err
	}
	if err := u.members.dir.RenameOrganization(ctx, keycloakID, alias, name); err != nil {
		return OrganizationView{}, fmt.Errorf("access: rename %q in the directory: %w", alias, err)
	}
	org, err = u.orgs.RenameOrganization(ctx, alias, name)
	if err != nil {
		return OrganizationView{}, fmt.Errorf("access: rename %q: %w", alias, err)
	}
	return u.describeOne(ctx, org)
}

// SetStatus suspends an organization for a reason its members see, or reactivates it. Suspension
// is the platform's alone: Keycloak still signs the members in, and every call in the
// organization's name answers 403.
func (u platformOrganizations) SetStatus(ctx context.Context, alias string, status domain.OrganizationStatus, reason string) (OrganizationView, error) {
	if err := u.authz.Require(ctx, ledger.PermPlatformOrganizations, ""); err != nil {
		return OrganizationView{}, err
	}
	if status != domain.OrganizationActive && status != domain.OrganizationSuspended {
		return OrganizationView{}, invalid("status must be active or suspended")
	}
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 500 {
		return OrganizationView{}, invalid("the reason must be at most 500 characters")
	}
	org, err := u.orgs.SetOrganizationStatus(ctx, alias, status, reason)
	if err != nil {
		return OrganizationView{}, fmt.Errorf("access: organization %q: %w", alias, err)
	}
	return u.describeOne(ctx, org)
}

// aliasPattern is the organizations_alias_format check of migration 00009: the alias appears in
// URLs and headers.
var aliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// validateOrganization checks the alias and returns the trimmed name.
func validateOrganization(alias, name string) (string, error) {
	if !aliasPattern.MatchString(alias) {
		return "", invalid("alias must be 1 to 63 lowercase letters, digits and dashes, starting with a letter or digit")
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return "", invalid("name must be 1 to 100 characters")
	}
	return name, nil
}

// validateInvite checks an invitation and returns it with the email in lower case, as Keycloak
// stores it.
func validateInvite(in Invite) (Invite, error) {
	addr, err := mail.ParseAddress(in.Email)
	if err != nil || addr.Address != strings.TrimSpace(in.Email) || len(addr.Address) > 254 {
		return Invite{}, invalid("email must be a plain address such as ada@example.com")
	}
	in.Email = strings.ToLower(addr.Address)
	in.FirstName, in.LastName = strings.TrimSpace(in.FirstName), strings.TrimSpace(in.LastName)
	if utf8.RuneCountInString(in.FirstName) > 100 || utf8.RuneCountInString(in.LastName) > 100 {
		return Invite{}, invalid("names must be at most 100 characters")
	}
	if in.Role == "" {
		in.Role = ledger.RoleMember
	}
	if !isOneOf(in.Role, ledger.OrganizationRoles) {
		return Invite{}, invalid("role must be member, admin or owner")
	}
	return in, nil
}

func isOneOf(r ledger.Role, roles []ledger.Role) bool {
	for _, x := range roles {
		if x == r {
			return true
		}
	}
	return false
}

func invalid(detail string) error {
	return ledger.NewProblem(ledger.ErrInvalid, "invalid-request", detail)
}
