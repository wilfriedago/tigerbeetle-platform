package application

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// noDirectory stands in for Keycloak's admin API when the API has no admin client
// (AUTH_ADMIN_CLIENT_ID): every call answers ErrUnavailable, so the rest of the API still runs.
type noDirectory struct{}

var _ domain.Directory = noDirectory{}

func unconfigured() error {
	return ledger.NewProblem(ledger.ErrUnavailable, "dependency-unavailable",
		"the identity provider's admin access is not configured")
}

func (noDirectory) Issuer() string { return "" }

func (noDirectory) OrganizationID(context.Context, string) (string, error) {
	return "", unconfigured()
}

func (noDirectory) CreateOrganization(context.Context, string, string) (string, error) {
	return "", unconfigured()
}

func (noDirectory) RenameOrganization(context.Context, string, string, string) error {
	return unconfigured()
}

func (noDirectory) PersonByEmail(context.Context, string) (domain.Person, error) {
	return domain.Person{}, unconfigured()
}

func (noDirectory) PersonBySubject(context.Context, string) (domain.Person, error) {
	return domain.Person{}, unconfigured()
}

func (noDirectory) MemberCount(context.Context, string) (int, error) { return 0, unconfigured() }

func (noDirectory) CreatePerson(context.Context, string, string, string) (domain.Person, error) {
	return domain.Person{}, unconfigured()
}

func (noDirectory) SendAccountSetup(context.Context, string) error { return unconfigured() }

func (noDirectory) Members(context.Context, string) ([]domain.Person, error) {
	return nil, unconfigured()
}

func (noDirectory) Member(context.Context, string, string) (domain.Person, error) {
	return domain.Person{}, unconfigured()
}

func (noDirectory) AddMember(context.Context, string, string) error    { return unconfigured() }
func (noDirectory) RemoveMember(context.Context, string, string) error { return unconfigured() }
func (noDirectory) Invite(context.Context, string, string) error       { return unconfigured() }

func (noDirectory) Invitations(context.Context, string) ([]domain.Invitation, error) {
	return nil, unconfigured()
}

func (noDirectory) ResendInvitation(context.Context, string, string) error { return unconfigured() }
func (noDirectory) DeleteInvitation(context.Context, string, string) error { return unconfigured() }
