// Package authntest provides authenticators for tests of the transports, which test what a call
// does once it is authenticated. The authentication itself is tested in internal/platform/authn
// and, end to end, with the in-process issuer of internal/platform/oidc/oidctest.
package authntest

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Principal is the caller Default authenticates every call as.
var Principal = ledger.Principal{Issuer: "test", Subject: "tester", Kind: ledger.KindPerson, Name: "tester", Organizations: []string{"default"}}

// Fixed authenticates every call as one principal acting for one tenant, whatever it sends.
type Fixed struct {
	Principal ledger.Principal
	Tenant    ledger.Tenant
}

// Default authenticates every call as Principal in the default organization.
func Default() Fixed {
	return Fixed{Principal: Principal, Tenant: ledger.DefaultTenant}
}

// Authenticate implements the transports' Authenticator.
func (f Fixed) Authenticate(ctx context.Context, _, _ string) (context.Context, error) {
	return ledger.WithTenant(ledger.WithPrincipal(ctx, f.Principal), f.Tenant), nil
}
