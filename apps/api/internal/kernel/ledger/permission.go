package ledger

// Permission is what a use case checks before it runs (ADR-0027). Ledger permissions apply to one
// ledger; organization permissions to the organization a call acts for.
type Permission string

// Ledger permissions.
const (
	// PermLedgerRead allows every query on a ledger: the ledger, its settings, accounts, balances,
	// history, transactions, holds, write requests, overview and search.
	PermLedgerRead Permission = "ledger.read"
	// PermMoneyWrite allows the writes that move money: transactions, reverts, holds, captures
	// and voids.
	PermMoneyWrite Permission = "money.write"
	// PermMetadataWrite allows replacing the metadata of an account or a transaction.
	PermMetadataWrite Permission = "metadata.write"
	// PermLedgerConfigure allows replacing the ledger's metadata, its rules and its event types.
	PermLedgerConfigure Permission = "ledger.configure"
	// PermLedgerMembers allows granting and revoking roles on the ledger.
	PermLedgerMembers Permission = "ledger.members"
)

// Organization permissions.
const (
	// PermOrgLedgers allows creating ledgers in the organization.
	PermOrgLedgers Permission = "org.ledgers"
	// PermOrgMembers allows inviting and removing members and granting organization roles.
	PermOrgMembers Permission = "org.members"
	// PermOrgCredentials allows creating and revoking API keys and machine clients.
	PermOrgCredentials Permission = "org.credentials"
	// PermOrgOperate allows reading the reconciler status of the organization.
	PermOrgOperate Permission = "org.operate"
	// PermPlatformOrganizations allows staff to create, rename, suspend and inspect
	// organizations. No role grants it; staff hold it through the identity provider.
	PermPlatformOrganizations Permission = "platform.organizations"
)

// OnLedger reports whether the permission applies to one ledger rather than to the organization.
func (p Permission) OnLedger() bool {
	switch p {
	case PermLedgerRead, PermMoneyWrite, PermMetadataWrite, PermLedgerConfigure, PermLedgerMembers:
		return true
	default:
		return false
	}
}

// Role is a named set of permissions, held in an organization or on one ledger.
type Role string

// Roles on one ledger.
const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	// RoleLedgerAdmin is the admin of one ledger.
	RoleLedgerAdmin Role = "admin"
)

// Roles in an organization.
const (
	// RoleMember sees the organization; ledger access comes only from ledger roles.
	RoleMember Role = "member"
	// RoleOrgAdmin holds every ledger permission on every ledger of the organization and the
	// organization permissions.
	RoleOrgAdmin Role = "admin"
	// RoleOwner is an organization admin who may also grant and remove the owner role.
	RoleOwner Role = "owner"
)

// The permissions of each role, written out in full so the table reads at a glance.
var (
	ledgerRead     = []Permission{PermLedgerRead}
	ledgerOperate  = []Permission{PermLedgerRead, PermMoneyWrite, PermMetadataWrite}
	ledgerAdmin    = []Permission{PermLedgerRead, PermMoneyWrite, PermMetadataWrite, PermLedgerConfigure, PermLedgerMembers}
	orgPermissions = []Permission{PermOrgLedgers, PermOrgMembers, PermOrgCredentials, PermOrgOperate}
)

// LedgerRoles lists the roles that may be held on one ledger, from least to most.
var LedgerRoles = []Role{RoleViewer, RoleOperator, RoleLedgerAdmin}

// OrganizationRoles lists the roles that may be held in an organization, from least to most.
var OrganizationRoles = []Role{RoleMember, RoleOrgAdmin, RoleOwner}

// LedgerRoleAllows reports whether a ledger role holds a permission.
func LedgerRoleAllows(r Role, p Permission) bool {
	switch r {
	case RoleViewer:
		return contains(ledgerRead, p)
	case RoleOperator:
		return contains(ledgerOperate, p)
	case RoleLedgerAdmin:
		return contains(ledgerAdmin, p)
	default:
		return false
	}
}

// OrganizationRoleAllows reports whether an organization role holds a permission, on every ledger
// of the organization for ledger permissions.
func OrganizationRoleAllows(r Role, p Permission) bool {
	switch r {
	case RoleOrgAdmin, RoleOwner:
		return contains(ledgerAdmin, p) || contains(orgPermissions, p)
	default:
		return false
	}
}

func contains(ps []Permission, p Permission) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}
