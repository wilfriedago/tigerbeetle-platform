// Package authn authenticates a call on any transport (ADR-0025, ADR-0031): it reads the credential
// and the organization the call names, and puts the principal and the tenant in the context. The
// HTTP middleware, the gRPC interceptors and the broker dispatcher all call Authenticate, so every
// transport accepts and refuses the same calls.
package authn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc"
)

// TokenVerifier checks an access token; internal/platform/oidc implements it.
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (ledger.Principal, error)
}

// Tenants resolves an organization alias; the access module implements it. It returns an error
// matching ledger.ErrNotFound for an unknown alias.
type Tenants interface {
	Tenant(ctx context.Context, alias string) (ledger.Tenant, error)
}

// Authenticator authenticates calls.
type Authenticator struct {
	tokens  TokenVerifier
	tenants Tenants
}

// New returns an authenticator.
func New(tokens TokenVerifier, tenants Tenants) *Authenticator {
	return &Authenticator{tokens: tokens, tenants: tenants}
}

var (
	errNoCredential = ledger.NewProblem(ledger.ErrUnauthenticated, "unauthenticated",
		"send an access token: Authorization: Bearer <token>")
	errBadCredential = ledger.NewProblem(ledger.ErrUnauthenticated, "unauthenticated",
		"the access token is not valid: it may have expired")
)

// Authenticate checks the authorization value ("Bearer <token>") and, when organization is not
// empty, resolves it. A person may only name an organization the identity provider lists them
// in; any other organization answers 404, as if it did not exist, so aliases are not disclosed.
// The returned context carries the principal, the tenant when one is named, and a logger that
// names both.
func (a *Authenticator) Authenticate(ctx context.Context, authorization, organization string) (context.Context, error) {
	raw := oidc.BearerToken(authorization)
	if raw == "" {
		return ctx, errNoCredential
	}
	p, err := a.tokens.Verify(ctx, raw)
	if err != nil {
		// The reason (expired, wrong audience, unknown key) goes to the log, not to the client.
		logging.From(ctx).InfoContext(ctx, "authentication failed", slog.Any("error", err))
		return ctx, errBadCredential
	}
	ctx = ledger.WithPrincipal(ctx, p)
	logger := logging.From(ctx).With(slog.String("principal", p.Subject))
	if organization != "" {
		t, err := a.tenant(ctx, p, organization)
		if err != nil {
			return ctx, err
		}
		ctx = ledger.WithTenant(ctx, t)
		logger = logger.With(slog.String("organization", t.Alias))
	}
	return logging.With(ctx, logger), nil
}

func (a *Authenticator) tenant(ctx context.Context, p ledger.Principal, alias string) (ledger.Tenant, error) {
	notFound := ledger.NewProblem(ledger.ErrNotFound, "organization-not-found", "no organization "+alias)
	if !p.MemberOf(alias) {
		return ledger.Tenant{}, notFound
	}
	t, err := a.tenants.Tenant(ctx, alias)
	if errors.Is(err, ledger.ErrNotFound) {
		return ledger.Tenant{}, notFound
	}
	if err != nil {
		return ledger.Tenant{}, fmt.Errorf("authn: %w", err)
	}
	return t, nil
}
