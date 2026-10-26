// Package graphql is the access module's GraphQL interface (ADR-0020, ADR-0027): the staff's
// organizations and the members of an organization, for the console. The registrations import
// neither gqlgen nor the generated executor; the resolvers have the signatures gqlgen generates
// from them and call the same use cases as REST.
package graphql

import (
	"context"
	"errors"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/dto"
	restv1 "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/rest/v1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/graphqlschema"
)

// Register adds the access queries and mutations to the schema.
func Register(s *graphqlschema.Schema) {
	graphqlschema.Describe[dto.Organization](s, "A tenant of the platform, as staff see it (ADR-0027).")
	graphqlschema.Describe[dto.Member](s, "A person of the organization with the roles the platform gives them (ADR-0028).")
	graphqlschema.Describe[dto.LedgerRole](s, "A role on one ledger.")
	graphqlschema.Describe[dto.Invitation](s, "An invitation the person has not yet accepted.")
	graphqlschema.Describe[dto.Owner](s, "An owner of an organization, or its first owner on the way in.")

	graphqlschema.Query[OrganizationsArgs, OrganizationConnection](s, "organizations", "Every organization in alias order. Staff only.")
	graphqlschema.Query[OrganizationArgs, *dto.Organization](s, "organization", "An organization by alias, or null. Staff only.")
	graphqlschema.Query[graphqlschema.NoArgs, []dto.Member](s, "members",
		"The members of the organization of the call and the people invited to it, in name order. Every member may read it.")

	graphqlschema.Mutation[CreateOrganizationArgs, dto.Organization](s, "createOrganization",
		"Creates an organization and invites its first owner. Retrying with the same alias finishes an interrupted creation. Staff only.")
	graphqlschema.Mutation[UpdateOrganizationArgs, dto.Organization](s, "updateOrganization",
		"Renames, suspends or reactivates an organization. Staff only.")
	graphqlschema.Mutation[InviteMemberArgs, dto.Member](s, "inviteMember",
		"Brings a person into the organization with an organization role: an invitation for a person with an account, an account and a setup email otherwise.")
	graphqlschema.Mutation[SetMemberRoleArgs, bool](s, "setMemberRole",
		"Replaces a person's organization role. Owner changes need an owner; the last owner stays.")
	graphqlschema.Mutation[MemberArgs, bool](s, "removeMember",
		"Ends a membership and removes the person's roles in the organization. Their account stays.")
	graphqlschema.Mutation[SetLedgerRoleArgs, bool](s, "setLedgerRole", "Gives a member a role on one ledger. Needs ledger.members.")
	graphqlschema.Mutation[RevokeLedgerRoleArgs, bool](s, "revokeLedgerRole", "Removes a person's role on one ledger. Needs ledger.members.")
	graphqlschema.Mutation[InvitationArgs, bool](s, "resendInvitation", "Sends an invitation again with a new expiry.")
	graphqlschema.Mutation[InvitationArgs, bool](s, "deleteInvitation", "Withdraws an invitation and the roles waiting for the person.")
	registerCredentials(s)
}

// OrganizationConnection is a page of organizations.
type OrganizationConnection struct {
	Nodes    []dto.Organization     `json:"nodes"`
	PageInfo graphqlschema.PageInfo `json:"pageInfo"`
}

// OrganizationsArgs pages the organizations.
type OrganizationsArgs struct {
	First *int    `json:"first" default:"25" doc:"Page size, 1 to 100."`
	After *string `json:"after" doc:"The endCursor of the previous page."`
}

// OrganizationArgs names an organization.
type OrganizationArgs struct {
	Alias string `json:"alias"`
}

// NewMemberInput names the person an invitation is for.
type NewMemberInput struct {
	Email     string  `json:"email"`
	FirstName *string `json:"firstName" doc:"Used when the person has no account yet."`
	LastName  *string `json:"lastName" doc:"Used when the person has no account yet."`
}

func (in NewMemberInput) dto() dto.NewMember {
	return dto.NewMember{Email: in.Email, FirstName: deref(in.FirstName), LastName: deref(in.LastName)}
}

// CreateOrganizationInput is the input of createOrganization.
type CreateOrganizationInput struct {
	Alias string         `json:"alias" doc:"Lowercase letters, digits and dashes, 1 to 63 characters. It cannot change."`
	Name  string         `json:"name"`
	Owner NewMemberInput `json:"owner"`
}

// CreateOrganizationArgs wraps the input.
type CreateOrganizationArgs struct {
	Input CreateOrganizationInput `json:"input"`
}

// UpdateOrganizationInput is the input of updateOrganization; absent fields stay.
type UpdateOrganizationInput struct {
	Alias  string  `json:"alias"`
	Name   *string `json:"name"`
	Status *string `json:"status" doc:"active or suspended."`
	Reason *string `json:"reason" doc:"Why the organization is suspended, shown to its members."`
}

// UpdateOrganizationArgs wraps the input.
type UpdateOrganizationArgs struct {
	Input UpdateOrganizationInput `json:"input"`
}

// InviteMemberInput is the input of inviteMember.
type InviteMemberInput struct {
	Person  NewMemberInput    `json:"person"`
	Role    *string           `json:"role" doc:"member, admin or owner; member when absent. Only an owner can invite an owner."`
	Ledgers []LedgerRoleInput `json:"ledgers" doc:"Roles on single ledgers, granted with the organization role."`
}

// LedgerRoleInput names a role on one ledger.
type LedgerRoleInput struct {
	Ledger string `json:"ledger"`
	Role   string `json:"role" doc:"viewer, operator or admin."`
}

// InviteMemberArgs wraps the input.
type InviteMemberArgs struct {
	Input InviteMemberInput `json:"input"`
}

// SetMemberRoleInput is the input of setMemberRole.
type SetMemberRoleInput struct {
	Subject string `json:"subject"`
	Role    string `json:"role" doc:"member, admin or owner."`
}

// SetMemberRoleArgs wraps the input.
type SetMemberRoleArgs struct {
	Input SetMemberRoleInput `json:"input"`
}

// MemberInput names a person.
type MemberInput struct {
	Subject string `json:"subject"`
}

// MemberArgs wraps the input.
type MemberArgs struct {
	Input MemberInput `json:"input"`
}

// SetLedgerRoleInput is the input of setLedgerRole.
type SetLedgerRoleInput struct {
	Subject string `json:"subject"`
	Ledger  string `json:"ledger"`
	Role    string `json:"role" doc:"viewer, operator or admin."`
}

// SetLedgerRoleArgs wraps the input.
type SetLedgerRoleArgs struct {
	Input SetLedgerRoleInput `json:"input"`
}

// RevokeLedgerRoleInput is the input of revokeLedgerRole.
type RevokeLedgerRoleInput struct {
	Subject string `json:"subject"`
	Ledger  string `json:"ledger"`
}

// RevokeLedgerRoleArgs wraps the input.
type RevokeLedgerRoleArgs struct {
	Input RevokeLedgerRoleInput `json:"input"`
}

// InvitationInput names an invitation.
type InvitationInput struct {
	ID string `json:"id"`
}

// InvitationArgs wraps the input.
type InvitationArgs struct {
	Input InvitationInput `json:"input"`
}

// Queries holds the access query resolvers.
type Queries struct {
	orgs        application.PlatformOrganizations
	members     application.Members
	credentials application.Credentials
}

// NewQueries builds the query resolvers.
func NewQueries(app application.Application) *Queries {
	return &Queries{orgs: app.PlatformOrganizations, members: app.Members, credentials: app.Credentials}
}

// Organizations resolves Query.organizations.
func (q *Queries) Organizations(ctx context.Context, first *int, after *string) (*OrganizationConnection, error) {
	views, next, err := q.orgs.List(ctx, deref(after), deref(first))
	if err != nil {
		return nil, err
	}
	out := &OrganizationConnection{Nodes: make([]dto.Organization, len(views)), PageInfo: graphqlschema.PageInfoOf(next)}
	for i := range views {
		out.Nodes[i] = dto.OrganizationFrom(views[i])
	}
	return out, nil
}

// Organization resolves Query.organization: null when there is none.
func (q *Queries) Organization(ctx context.Context, alias string) (*dto.Organization, error) {
	v, err := q.orgs.Get(ctx, alias)
	if errors.Is(err, ledger.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o := dto.OrganizationFrom(v)
	return &o, nil
}

// Members resolves Query.members.
func (q *Queries) Members(ctx context.Context) ([]dto.Member, error) {
	views, err := q.members.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Member, len(views))
	for i := range views {
		out[i] = dto.MemberFrom(views[i])
	}
	return out, nil
}

// Mutations holds the access mutation resolvers.
type Mutations struct {
	orgs        application.PlatformOrganizations
	members     application.Members
	credentials application.Credentials
}

// NewMutations builds the mutation resolvers.
func NewMutations(app application.Application) *Mutations {
	return &Mutations{orgs: app.PlatformOrganizations, members: app.Members, credentials: app.Credentials}
}

// CreateOrganization resolves Mutation.createOrganization.
func (m *Mutations) CreateOrganization(ctx context.Context, input CreateOrganizationInput) (*dto.Organization, error) {
	v, err := m.orgs.Create(ctx, application.CreateOrganization{
		Alias: input.Alias, Name: input.Name, Owner: dto.InviteFrom(input.Owner.dto(), "", nil),
	})
	if err != nil {
		return nil, err
	}
	o := dto.OrganizationFrom(v)
	return &o, nil
}

// UpdateOrganization resolves Mutation.updateOrganization.
func (m *Mutations) UpdateOrganization(ctx context.Context, input UpdateOrganizationInput) (*dto.Organization, error) {
	v, err := restv1.UpdateOrganization(ctx, m.orgs, input.Alias, dto.UpdateOrganizationRequest{Name: input.Name, Status: input.Status, Reason: input.Reason})
	if err != nil {
		return nil, err
	}
	o := dto.OrganizationFrom(v)
	return &o, nil
}

// InviteMember resolves Mutation.inviteMember.
func (m *Mutations) InviteMember(ctx context.Context, input InviteMemberInput) (*dto.Member, error) {
	ledgers := make([]dto.LedgerRoleSpec, len(input.Ledgers))
	for i, l := range input.Ledgers {
		ledgers[i] = dto.LedgerRoleSpec{Ledger: l.Ledger, Role: l.Role}
	}
	v, err := m.members.Invite(ctx, dto.InviteFrom(input.Person.dto(), deref(input.Role), ledgers))
	if err != nil {
		return nil, err
	}
	out := dto.MemberFrom(v)
	return &out, nil
}

// SetMemberRole resolves Mutation.setMemberRole.
func (m *Mutations) SetMemberRole(ctx context.Context, input SetMemberRoleInput) (bool, error) {
	return done(m.members.SetRole(ctx, input.Subject, ledger.Role(input.Role)))
}

// RemoveMember resolves Mutation.removeMember.
func (m *Mutations) RemoveMember(ctx context.Context, input MemberInput) (bool, error) {
	return done(m.members.Remove(ctx, input.Subject))
}

// SetLedgerRole resolves Mutation.setLedgerRole.
func (m *Mutations) SetLedgerRole(ctx context.Context, input SetLedgerRoleInput) (bool, error) {
	return done(m.members.SetLedgerRole(ctx, input.Subject, input.Ledger, ledger.Role(input.Role)))
}

// RevokeLedgerRole resolves Mutation.revokeLedgerRole.
func (m *Mutations) RevokeLedgerRole(ctx context.Context, input RevokeLedgerRoleInput) (bool, error) {
	return done(m.members.RevokeLedgerRole(ctx, input.Subject, input.Ledger))
}

// ResendInvitation resolves Mutation.resendInvitation.
func (m *Mutations) ResendInvitation(ctx context.Context, input InvitationInput) (bool, error) {
	return done(m.members.ResendInvitation(ctx, input.ID))
}

// DeleteInvitation resolves Mutation.deleteInvitation.
func (m *Mutations) DeleteInvitation(ctx context.Context, input InvitationInput) (bool, error) {
	return done(m.members.DeleteInvitation(ctx, input.ID))
}

func done(err error) (bool, error) {
	return err == nil, err
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
