// Package v1 is version 1 of the access module's REST interface (ADR-0027, ADR-0028): the staff's
// organization operations and the members of an organization, written for Huma (ADR-0018). The
// handlers decode, call one use case and map the result with package dto.
package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// Deps holds the use cases the handlers call. cmd/openapi passes the zero value.
type Deps struct {
	PlatformOrganizations application.PlatformOrganizations
	Members               application.Members
	Credentials           application.Credentials
}

type handlers struct {
	Deps
}

// Register adds the operations of the access module to api.
func Register(api huma.API, deps Deps) {
	h := &handlers{Deps: deps}
	registerOrganizations(api, h)
	registerMembers(api, h)
	registerCredentials(api, h)
}

// OrganizationPath names an organization in the staff's paths.
type OrganizationPath struct {
	Organization string `path:"organization" doc:"The organization's alias."`
}

// SubjectPath names a person by the subject of their tokens.
type SubjectPath struct {
	Subject string `path:"subject" doc:"The identity provider's ID of the person."`
}

// InvitationPath names an invitation.
type InvitationPath struct {
	Invitation string `path:"invitation" doc:"The invitation's ID."`
}

func registerOrganizations(api huma.API, h *handlers) {
	tags := []string{"organizations"}
	restapi.RegisterPlatform(api, huma.Operation{
		OperationID: "listOrganizations", Method: http.MethodGet, Path: "/v1/organizations", Tags: tags,
		Summary:     "List organizations",
		Description: "Every organization of the platform, in alias order. Staff only.",
		Parameters:  []*huma.Param{restapi.CursorParam()},
		Responses:   restapi.Success(http.StatusOK, "A page of organizations."),
	}, h.listOrganizations)

	restapi.RegisterPlatform(api, huma.Operation{
		OperationID: "createOrganization", Method: http.MethodPost, Path: "/v1/organizations", Tags: tags,
		Summary: "Create an organization",
		Description: "Creates the organization in the identity provider and the platform, and invites its first\n" +
			"owner. Retrying with the same alias finishes an interrupted creation; an organization that\n" +
			"already has an owner answers 409. Staff only.\n",
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The organization was created and its owner invited."),
	}, h.createOrganization)

	restapi.RegisterPlatform(api, huma.Operation{
		OperationID: "getOrganization", Method: http.MethodGet, Path: "/v1/organizations/{organization}", Tags: tags,
		Summary:     "Get an organization",
		Description: "One organization. Staff only.",
		Responses:   restapi.Success(http.StatusOK, "The organization."),
	}, h.getOrganization)

	restapi.RegisterPlatform(api, huma.Operation{
		OperationID: "updateOrganization", Method: http.MethodPatch, Path: "/v1/organizations/{organization}", Tags: tags,
		Summary: "Rename, suspend or reactivate an organization",
		Description: "Changes the fields the body names. A suspended organization answers 403\n" +
			"`organization-suspended` to every call made in its name; its members can still sign in.\n" +
			"Staff only.\n",
		Responses: restapi.Success(http.StatusOK, "The organization."),
	}, h.updateOrganization)
}

func (h *handlers) listOrganizations(ctx context.Context, in *struct{ restapi.Page }) (*restapi.Body[dto.OrganizationList], error) {
	views, next, err := h.PlatformOrganizations.List(ctx, in.Cursor, in.Limit)
	if err != nil {
		return nil, err
	}
	out := dto.OrganizationList{Data: make([]dto.Organization, len(views)), Next: restapi.Optional(next)}
	for i := range views {
		out.Data[i] = dto.OrganizationFrom(views[i])
	}
	return &restapi.Body[dto.OrganizationList]{Body: out}, nil
}

func (h *handlers) createOrganization(ctx context.Context, in *restapi.Body[dto.CreateOrganizationRequest]) (*restapi.Body[dto.Organization], error) {
	v, err := h.PlatformOrganizations.Create(ctx, application.CreateOrganization{
		Alias: in.Body.Alias, Name: in.Body.Name, Owner: dto.InviteFrom(in.Body.Owner, "", nil),
	})
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Organization]{Body: dto.OrganizationFrom(v)}, nil
}

func (h *handlers) getOrganization(ctx context.Context, in *OrganizationPath) (*restapi.Body[dto.Organization], error) {
	v, err := h.PlatformOrganizations.Get(ctx, in.Organization)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Organization]{Body: dto.OrganizationFrom(v)}, nil
}

func (h *handlers) updateOrganization(ctx context.Context, in *struct {
	OrganizationPath
	Body dto.UpdateOrganizationRequest
},
) (*restapi.Body[dto.Organization], error) {
	v, err := UpdateOrganization(ctx, h.PlatformOrganizations, in.Organization, in.Body)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Organization]{Body: dto.OrganizationFrom(v)}, nil
}

// UpdateOrganization applies the fields of an update, the name first. GraphQL shares it.
func UpdateOrganization(ctx context.Context, u application.PlatformOrganizations, alias string, in dto.UpdateOrganizationRequest) (application.OrganizationView, error) {
	v, err := u.Get(ctx, alias)
	if err != nil {
		return v, err
	}
	if in.Name != nil {
		if v, err = u.Rename(ctx, alias, *in.Name); err != nil {
			return v, err
		}
	}
	if in.Status != nil {
		reason := ""
		if in.Reason != nil {
			reason = *in.Reason
		}
		if v, err = u.SetStatus(ctx, alias, dto.StatusOf(*in.Status), reason); err != nil {
			return v, err
		}
	}
	return v, nil
}

func registerMembers(api huma.API, h *handlers) {
	tags := []string{"members"}
	restapi.Register(api, huma.Operation{
		OperationID: "listMembers", Method: http.MethodGet, Path: "/v1/members", Tags: tags,
		Summary: "List members",
		Description: "The members of the organization and the people invited to it, in name order, with their\n" +
			"organization and ledger roles. Every member of the organization may read it.\n",
		Responses: restapi.Success(http.StatusOK, "The members."),
	}, h.listMembers)

	restapi.Register(api, huma.Operation{
		OperationID: "inviteMember", Method: http.MethodPost, Path: "/v1/members", Tags: tags,
		Summary: "Invite a member",
		Description: "Brings a person into the organization with an organization role. A person with an account\n" +
			"gets an invitation by email and becomes a member by following it; a person without one gets\n" +
			"an account and an email to verify the address and set a password. Inviting someone already\n" +
			"setting up their account sends that email again; a member who holds a role answers 409.\n" +
			"Needs `org.members`.\n",
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The person was invited."),
	}, h.inviteMember)

	restapi.Register(api, huma.Operation{
		OperationID: "setMemberRole", Method: http.MethodPut, Path: "/v1/members/{subject}/role", Tags: tags,
		Summary:       "Set a member's organization role",
		Description:   "Replaces the person's organization role. Needs `org.members`; owner changes need an owner.",
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The role was set."),
	}, h.setMemberRole)

	restapi.Register(api, huma.Operation{
		OperationID: "removeMember", Method: http.MethodDelete, Path: "/v1/members/{subject}", Tags: tags,
		Summary: "Remove a member",
		Description: "Ends the membership and removes every role the person holds in the organization. Their\n" +
			"account and other organizations stay. Needs `org.members`, except to leave: anyone may remove\n" +
			"themselves. Removing an owner needs an owner, and the last owner stays.\n",
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The member was removed."),
	}, h.removeMember)

	restapi.Register(api, huma.Operation{
		OperationID: "setLedgerRole", Method: http.MethodPut, Path: "/v1/ledgers/{ledger}/members/{subject}", Tags: tags,
		Summary:       "Set a ledger role",
		Description:   "Gives a member of the organization a role on the ledger, replacing the one they held. Needs `ledger.members`.",
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The role was set."),
	}, h.setLedgerRole)

	restapi.Register(api, huma.Operation{
		OperationID: "revokeLedgerRole", Method: http.MethodDelete, Path: "/v1/ledgers/{ledger}/members/{subject}", Tags: tags,
		Summary:       "Revoke a ledger role",
		Description:   "Removes the person's role on the ledger. Needs `ledger.members`.",
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The role was revoked."),
	}, h.revokeLedgerRole)

	restapi.Register(api, huma.Operation{
		OperationID: "resendInvitation", Method: http.MethodPost, Path: "/v1/invitations/{invitation}/resend", Tags: tags,
		Summary:       "Resend an invitation",
		Description:   "Sends the invitation again with a new expiry. Needs `org.members`.",
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The invitation was sent again."),
	}, h.resendInvitation)

	restapi.Register(api, huma.Operation{
		OperationID: "deleteInvitation", Method: http.MethodDelete, Path: "/v1/invitations/{invitation}", Tags: tags,
		Summary:       "Withdraw an invitation",
		Description:   "Withdraws the invitation and the roles waiting for the person. Needs `org.members`.",
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The invitation was withdrawn."),
	}, h.deleteInvitation)
}

func (h *handlers) listMembers(ctx context.Context, _ *struct{}) (*restapi.Body[dto.MemberList], error) {
	views, err := h.Members.List(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.MemberList{Data: make([]dto.Member, len(views))}
	for i := range views {
		out.Data[i] = dto.MemberFrom(views[i])
	}
	return &restapi.Body[dto.MemberList]{Body: out}, nil
}

func (h *handlers) inviteMember(ctx context.Context, in *restapi.Body[dto.InviteMemberRequest]) (*restapi.Body[dto.Member], error) {
	v, err := h.Members.Invite(ctx, dto.InviteFrom(in.Body.NewMember, in.Body.Role, in.Body.Ledgers))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Member]{Body: dto.MemberFrom(v)}, nil
}

func (h *handlers) setMemberRole(ctx context.Context, in *struct {
	SubjectPath
	Body dto.OrganizationRoleRequest
},
) (*struct{}, error) {
	return &struct{}{}, h.Members.SetRole(ctx, in.Subject, ledger.Role(in.Body.Role))
}

func (h *handlers) removeMember(ctx context.Context, in *SubjectPath) (*struct{}, error) {
	return &struct{}{}, h.Members.Remove(ctx, in.Subject)
}

func (h *handlers) setLedgerRole(ctx context.Context, in *struct {
	restapi.LedgerPath
	SubjectPath
	Body dto.LedgerRoleRequest
},
) (*struct{}, error) {
	return &struct{}{}, h.Members.SetLedgerRole(ctx, in.Subject, string(in.Ledger), ledger.Role(in.Body.Role))
}

func (h *handlers) revokeLedgerRole(ctx context.Context, in *struct {
	restapi.LedgerPath
	SubjectPath
},
) (*struct{}, error) {
	return &struct{}{}, h.Members.RevokeLedgerRole(ctx, in.Subject, string(in.Ledger))
}

func (h *handlers) resendInvitation(ctx context.Context, in *InvitationPath) (*struct{}, error) {
	return &struct{}{}, h.Members.ResendInvitation(ctx, in.Invitation)
}

func (h *handlers) deleteInvitation(ctx context.Context, in *InvitationPath) (*struct{}, error) {
	return &struct{}{}, h.Members.DeleteInvitation(ctx, in.Invitation)
}
