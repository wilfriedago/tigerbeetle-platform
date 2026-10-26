package bootstrap

import (
	"context"
	"strings"

	accessapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/web"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/sealed"
)

// console is the console's sign-in (ADR-0030), nil when AUTH_CLIENT_ID is empty.
type console struct {
	sessions accessapp.Sessions
	web      web.Config
}

// consoleSignIn builds the backend-for-frontend of the console from the verifier's provider.
func consoleSignIn(cfg config.Config, verifier *oidc.Verifier, m modules) (*console, error) {
	if cfg.AuthClientID == "" {
		return nil, nil
	}
	box, err := sealed.New(cfg.AuthSessionKey)
	if err != nil {
		return nil, err
	}
	client, err := verifier.Client(oidc.ClientConfig{
		ClientID: cfg.AuthClientID, ClientSecret: cfg.AuthClientSecret, RedirectURL: cfg.AuthRedirectURL,
		// organization:* lists every organization of the person in the token (ADR-0027).
		Scopes: []string{"organization:*"},
	})
	if err != nil {
		return nil, err
	}
	sessions := accessapp.NewSessions(m.accessDeps, accessapp.SessionDeps{
		IdentityProvider: identityProvider{client: client}, Tokens: verifier, Sealer: box,
	})
	return &console{sessions: sessions, web: web.Config{
		Secure: cfg.AuthCookieSecure, ConsoleOrigin: cfg.AuthConsoleOrigin,
		// Keycloak's account console lives under the realm; another provider would be configured.
		AccountURL: strings.TrimSuffix(cfg.AuthIssuerURL, "/") + "/account",
	}}, nil
}

// identityProvider adapts the platform's OIDC client to the access module's port.
type identityProvider struct {
	client *oidc.Client
}

func (p identityProvider) AuthCodeURL(state, nonce, verifier string) string {
	return p.client.AuthCodeURL(state, nonce, verifier)
}

func (p identityProvider) Exchange(ctx context.Context, code, verifier, nonce string) (accessapp.Tokens, error) {
	t, err := p.client.Exchange(ctx, code, verifier, nonce)
	return accessapp.Tokens(t), err
}

func (p identityProvider) Refresh(ctx context.Context, refreshToken string) (accessapp.Tokens, error) {
	t, err := p.client.Refresh(ctx, refreshToken)
	return accessapp.Tokens(t), err
}

func (p identityProvider) EndSessionURL(idTokenHint, postLogoutRedirect string) string {
	return p.client.EndSessionURL(idTokenHint, postLogoutRedirect)
}
