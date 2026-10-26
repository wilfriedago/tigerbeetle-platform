package authn_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc/oidctest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

// tenants knows acme and a suspended globex.
type tenants struct{}

func (tenants) Tenant(_ context.Context, alias string) (ledger.Tenant, error) {
	switch alias {
	case "acme":
		return ledger.Tenant{ID: ledger.ID{15: 7}, Alias: "acme"}, nil
	case "globex":
		return ledger.Tenant{}, ledger.NewProblem(ledger.ErrForbidden, "organization-suspended", "suspended")
	default:
		return ledger.Tenant{}, ledger.ErrNotFound
	}
}

func TestAuthenticate(t *testing.T) {
	iss := oidctest.New(t)
	verifier, err := oidc.New(context.Background(), oidc.Config{IssuerURL: iss.URL, Audience: oidctest.Audience, OrganizationClaim: "organization"})
	if err != nil {
		t.Fatal(err)
	}
	auth := authn.New(verifier, tenants{}, nil)
	member := "Bearer " + iss.Person(t, "ada", "acme", "globex", "gone")

	tests := []struct {
		name          string
		authorization string
		organization  string
		wantReason    string
		wantTenant    string
	}{
		{name: "no credential", wantReason: "unauthenticated"},
		{name: "not a bearer", authorization: "Basic YWRhOnB3", wantReason: "unauthenticated"},
		{name: "bad token", authorization: "Bearer nope", wantReason: "unauthenticated"},
		{name: "no organization named", authorization: member},
		{name: "a member's organization", authorization: member, organization: "acme", wantTenant: "acme"},
		{name: "an organization the person is not in", authorization: "Bearer " + iss.Person(t, "bob", "acme"), organization: "globex", wantReason: "organization-not-found"},
		{name: "a member of an unknown organization", authorization: member, organization: "gone", wantReason: "organization-not-found"},
		{name: "a suspended organization", authorization: member, organization: "globex", wantReason: "organization-suspended"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := auth.Authenticate(context.Background(), tt.authorization, tt.organization)
			if tt.wantReason != "" {
				var p *ledger.Problem
				if !errors.As(err, &p) || p.Reason != tt.wantReason {
					t.Fatalf("error = %v, want reason %s", err, tt.wantReason)
				}
				if tt.wantReason == "unauthenticated" && problem.From(err).Status != 401 {
					t.Fatalf("status = %d, want 401", problem.From(err).Status)
				}
				return
			}
			if err != nil {
				t.Fatalf("Authenticate: %v", err)
			}
			if p, ok := ledger.PrincipalFrom(ctx); !ok || p.Subject == "" {
				t.Fatalf("no principal in the context: %+v", p)
			}
			got, ok := ledger.TenantFrom(ctx)
			if tt.wantTenant == "" {
				if ok {
					t.Fatalf("tenant = %+v, want none", got)
				}
				return
			}
			if got.Alias != tt.wantTenant {
				t.Fatalf("tenant = %+v, want %s", got, tt.wantTenant)
			}
		})
	}
}
