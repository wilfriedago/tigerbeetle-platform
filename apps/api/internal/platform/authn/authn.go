// Package authn authenticates a call on any transport (ADR-0026, ADR-0032): it reads the credential
// and the organization the call names, and puts the principal and the tenant in the context. The
// HTTP middleware, the gRPC interceptors and the broker dispatcher all call Authenticate, so every
// transport accepts and refuses the same calls.
package authn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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

// Credentials resolves the platform's own credentials (ADR-0029); the access module implements
// it. An API key names its organization; a machine token's client does.
type Credentials interface {
	APIKey(ctx context.Context, raw string) (ledger.Principal, ledger.Tenant, error)
	MachineTenant(ctx context.Context, p ledger.Principal) (ledger.Tenant, error)
}

// Authenticator authenticates calls.
type Authenticator struct {
	tokens      TokenVerifier
	tenants     Tenants
	credentials Credentials
}

// New returns an authenticator. Without credentials, API keys and machine tokens are refused.
func New(tokens TokenVerifier, tenants Tenants, credentials Credentials) *Authenticator {
	return &Authenticator{tokens: tokens, tenants: tenants, credentials: credentials}
}

// apiKeyPrefix starts every API key, tbp_<id>_<secret>; any other bearer value is a token.
const apiKeyPrefix = "tbp_"

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
	if strings.HasPrefix(raw, apiKeyPrefix) {
		if a.credentials == nil {
			return ctx, errBadCredential
		}
		p, t, err := a.credentials.APIKey(ctx, raw)
		if err != nil {
			logging.From(ctx).InfoContext(ctx, "authentication failed", slog.Any("error", err))
			return ctx, err
		}
		return a.owned(ctx, p, t, organization)
	}
	p, err := a.tokens.Verify(ctx, raw)
	if err != nil {
		// The reason (expired, wrong audience, unknown key) goes to the log, not to the client.
		logging.From(ctx).InfoContext(ctx, "authentication failed", slog.Any("error", err))
		return ctx, errBadCredential
	}
	if p.Kind == ledger.KindMachine {
		if a.credentials == nil {
			return ctx, errBadCredential
		}
		t, err := a.credentials.MachineTenant(ctx, p)
		if err != nil {
			logging.From(ctx).InfoContext(ctx, "authentication failed", slog.Any("error", err))
			return ctx, err
		}
		return a.owned(ctx, p, t, organization)
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

// owned puts a credential that belongs to one organization in the context: an API key or a
// machine client acts for its own organization, and naming another is 403 (plan section 5.2).
func (a *Authenticator) owned(ctx context.Context, p ledger.Principal, t ledger.Tenant, organization string) (context.Context, error) {
	if organization != "" && organization != t.Alias {
		return ctx, ledger.NewProblem(ledger.ErrForbidden, "forbidden", "this credential belongs to the organization "+t.Alias)
	}
	ctx = ledger.WithTenant(ledger.WithPrincipal(ctx, p), t)
	logger := logging.From(ctx).With(slog.String("principal", string(p.Kind)+":"+p.Subject), slog.String("organization", t.Alias))
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
