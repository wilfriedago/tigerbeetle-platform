package ledger

import "context"

// PrincipalKind says what kind of caller a principal is. It is for logs and display; decisions
// rest on grants (ADR-0027), never on the kind alone.
type PrincipalKind string

// The kinds of principal.
const (
	// KindPerson is someone signed in through the identity provider.
	KindPerson PrincipalKind = "person"
	// KindMachine is an OAuth machine client using client credentials (ADR-0028).
	KindMachine PrincipalKind = "machine"
	// KindAPIKey is a platform API key (ADR-0028).
	KindAPIKey PrincipalKind = "api-key"
	// KindSystem is the platform itself: the reconciler and the CDC relay.
	KindSystem PrincipalKind = "system"
)

// Principal is the caller of a call, as the API knows it after authentication (ADR-0025). Issuer
// and Subject together identify it; an email never does, because emails change and two issuers
// can vouch for the same one.
type Principal struct {
	Issuer  string
	Subject string
	Kind    PrincipalKind
	// Name is for logs and the console, never for decisions.
	Name string
	// Staff marks the people who run the platform (ADR-0027). Staff manage organizations; they
	// have no permission inside one without a grant.
	Staff bool
	// Organizations lists the aliases of the organizations the identity provider says the
	// principal belongs to (Keycloak's organization claim, ADR-0026).
	Organizations []string
}

// MemberOf reports whether the identity provider lists the principal in the organization.
func (p Principal) MemberOf(alias string) bool {
	for _, a := range p.Organizations {
		if a == alias {
			return true
		}
	}
	return false
}

// System is the principal of the platform's own workers. Only internal/bootstrap sets it.
var System = Principal{Issuer: "tbp:system", Subject: "system", Kind: KindSystem, Name: "system"}

type principalKey struct{}

// WithPrincipal returns a copy of ctx that carries the principal.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the principal carried by ctx, and false when the call is anonymous.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}
