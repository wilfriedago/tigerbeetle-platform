package ledger

import "context"

// Tenant is the organization a call acts for (ADR-0026). Every ledger belongs to exactly one
// organization, and ledger names are unique inside it, so a ledger is found by the tenant and its
// name together. The tenant is resolved once per call, at the entry point, and travels in the
// context.
type Tenant struct {
	// ID is the platform's identifier of the organization, the key its tables point at.
	ID ID
	// Alias is the organization's alias, unique and immutable in Keycloak, and the value of the
	// X-Organization header.
	Alias string
}

// DefaultOrganizationID is the organization that pre-alpha data was moved into when ledgers
// became per organization (ADR-0026, migration 00009). Its alias is "default".
var DefaultOrganizationID = ID{15: 1}

// DefaultTenant is the tenant of DefaultOrganizationID.
var DefaultTenant = Tenant{ID: DefaultOrganizationID, Alias: "default"}

// tenantKey is the context key of the tenant. It is unexported, so no other package can set or
// overwrite the value except through WithTenant.
type tenantKey struct{}

// WithTenant returns a copy of ctx that carries the tenant.
func WithTenant(ctx context.Context, t Tenant) context.Context {
	return context.WithValue(ctx, tenantKey{}, t)
}

// TenantFrom returns the tenant carried by ctx, and false when there is none.
func TenantFrom(ctx context.Context) (Tenant, bool) {
	t, ok := ctx.Value(tenantKey{}).(Tenant)
	return t, ok
}

// RequireTenant returns the tenant carried by ctx. A call about a ledger that names no
// organization is a client error: repositories fail closed instead of reading every tenant.
func RequireTenant(ctx context.Context) (Tenant, error) {
	t, ok := TenantFrom(ctx)
	if !ok || t.ID.IsZero() {
		return Tenant{}, ErrNoOrganization
	}
	return t, nil
}

// ErrNoOrganization is the answer to a call about a ledger that names no organization.
var ErrNoOrganization = NewProblem(ErrInvalid, "organization-required",
	"name the organization: the X-Organization header, x-organization gRPC metadata, or the organization message header")
