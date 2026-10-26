package oidc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc/oidctest"
)

func newVerifier(t *testing.T, iss *oidctest.Issuer) *oidc.Verifier {
	t.Helper()
	v, err := oidc.New(context.Background(), oidc.Config{
		IssuerURL: iss.URL, Audience: oidctest.Audience,
		StaffClaim: "roles", StaffValue: "tbp-staff", OrganizationClaim: "organization",
	})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVerify(t *testing.T) {
	iss := oidctest.New(t)
	other := oidctest.New(t)
	v := newVerifier(t, iss)
	expired := time.Now().Add(-2 * time.Hour).Unix()
	tests := []struct {
		name  string
		token string
		want  ledger.Principal
		err   error
	}{
		{
			name:  "person in two organizations",
			token: iss.Person(t, "ada", "acme", "globex"),
			want:  ledger.Principal{Issuer: iss.URL, Subject: "ada", Kind: ledger.KindPerson, Name: "ada", Organizations: []string{"acme", "globex"}},
		},
		{
			name:  "organization claim keyed by alias",
			token: iss.Token(t, map[string]any{"sub": "bob", "organization": map[string]any{"acme": map[string]any{"id": "x"}}}),
			want:  ledger.Principal{Issuer: iss.URL, Subject: "bob", Kind: ledger.KindPerson, Organizations: []string{"acme"}},
		},
		{
			name:  "staff in no organization",
			token: iss.Staff(t, "ops"),
			want:  ledger.Principal{Issuer: iss.URL, Subject: "ops", Kind: ledger.KindPerson, Name: "ops", Staff: true},
		},
		{name: "another audience", token: iss.Token(t, map[string]any{"sub": "a", "aud": "tbp-console"}), err: ledger.ErrUnauthenticated},
		{name: "expired", token: iss.Token(t, map[string]any{"sub": "a", "exp": expired}), err: ledger.ErrUnauthenticated},
		{name: "another issuer's key", token: other.Token(t, map[string]any{"sub": "a", "iss": iss.URL}), err: ledger.ErrUnauthenticated},
		{name: "not a token", token: "abc", err: ledger.ErrUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v.Verify(context.Background(), tt.token)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("Verify error = %v, want %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("principal (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	tests := map[string]string{
		"Bearer abc":   "abc",
		"bearer  abc ": "abc",
		"Basic abc":    "",
		"Bearer":       "",
		"":             "",
	}
	for in, want := range tests {
		if got := oidc.BearerToken(in); got != want {
			t.Errorf("BearerToken(%q) = %q, want %q", in, got, want)
		}
	}
}
