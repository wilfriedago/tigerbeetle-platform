// Package oidc verifies the access tokens of the identity provider (ADR-0026) and maps their claims
// to a ledger.Principal. It speaks standard OpenID Connect only: discovery from the issuer, the
// provider's published keys, then the signature, issuer, audience and lifetime of each token.
// Nothing here names Keycloak; where a provider puts roles and organizations is configured.
package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Config says which tokens the API accepts and where their claims live.
type Config struct {
	// IssuerURL is the issuer the tokens name in their iss claim.
	IssuerURL string
	// DiscoveryURL is where the API reads the discovery document and the keys, when it reaches
	// the provider at another address than browsers do (inside compose, for instance). Empty
	// means IssuerURL.
	DiscoveryURL string
	// Audience is the value the aud claim must contain: the API's own identifier.
	Audience string
	// StaffClaim and StaffValue mark the platform's staff: a top-level claim holding a string or
	// a list of strings, and the value it must hold.
	StaffClaim string
	StaffValue string
	// OrganizationClaim holds the aliases of the principal's organizations: a list of strings, a
	// single string, or an object keyed by alias (Keycloak's organization claim with ids).
	OrganizationClaim string
	// HTTPClient fetches discovery and keys. Nil means http.DefaultClient.
	HTTPClient *http.Client
}

// clockSkew is how long after its expiry a token is still accepted, to absorb ordinary drift
// between the provider's clock and the API's.
const clockSkew = time.Minute

// Verifier checks access tokens.
type Verifier struct {
	verifier *gooidc.IDTokenVerifier
	provider *gooidc.Provider
	// ctx carries the HTTP client and the issuer override for later calls to the provider.
	ctx context.Context
	cfg Config
	// discovery is the address the provider is read at, for the readiness probe.
	discovery string
}

// New reads the provider's discovery document and returns a verifier. The keys are fetched on
// first use, cached, and fetched again when a token names a key the cache does not hold, which is
// how key rotation works without a restart.
func New(ctx context.Context, cfg Config) (*Verifier, error) {
	if cfg.IssuerURL == "" || cfg.Audience == "" {
		return nil, errors.New("oidc: issuer URL and audience are required")
	}
	// The provider keeps this context for every later key fetch, so it must outlive ctx.
	ctx = context.WithoutCancel(ctx)
	if cfg.HTTPClient != nil {
		ctx = gooidc.ClientContext(ctx, cfg.HTTPClient)
	}
	discovery := cfg.IssuerURL
	if cfg.DiscoveryURL != "" && cfg.DiscoveryURL != cfg.IssuerURL {
		// Discovery at one address, tokens naming another: go-oidc then checks the issuer of
		// each token against IssuerURL instead of the discovery address.
		ctx = gooidc.InsecureIssuerURLContext(ctx, cfg.IssuerURL)
		discovery = cfg.DiscoveryURL
	}
	provider, err := gooidc.NewProvider(ctx, discovery)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery at %s: %w", discovery, err)
	}
	v := provider.Verifier(&gooidc.Config{
		// go-oidc compares ClientID with the aud claim; for an access token that is the API.
		ClientID:             cfg.Audience,
		SupportedSigningAlgs: []string{gooidc.RS256, gooidc.RS384, gooidc.RS512, gooidc.ES256, gooidc.ES384, gooidc.ES512, gooidc.PS256, gooidc.PS384, gooidc.PS512},
		Now:                  func() time.Time { return time.Now().Add(-clockSkew) },
	})
	return &Verifier{verifier: v, provider: provider, ctx: ctx, cfg: cfg, discovery: discovery}, nil
}

// Ready probes the provider's discovery document, for /readyz. When the provider does not
// answer, tokens signed with keys already fetched still verify, but nobody can sign in and a new
// signing key cannot be fetched, so it is reported, not fatal.
func (v *Verifier) Ready(ctx context.Context) error {
	client := v.cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(v.discovery, "/")+"/.well-known/openid-configuration", http.NoBody)
	if err != nil {
		return fmt.Errorf("oidc: probe: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("oidc: probe: %w", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("oidc: probe: status %d", res.StatusCode)
	}
	return nil
}

// Verify checks a raw access token and returns its principal. Every failure is ErrUnauthenticated;
// the reason goes to the logs only, through the wrapped error.
func (v *Verifier) Verify(ctx context.Context, raw string) (ledger.Principal, error) {
	tok, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return ledger.Principal{}, fmt.Errorf("%w: %w", ledger.ErrUnauthenticated, err)
	}
	var claims map[string]json.RawMessage
	if err := tok.Claims(&claims); err != nil {
		return ledger.Principal{}, fmt.Errorf("%w: claims: %w", ledger.ErrUnauthenticated, err)
	}
	p := ledger.Principal{
		Issuer:        tok.Issuer,
		Subject:       tok.Subject,
		Kind:          ledger.KindPerson,
		Name:          firstString(claims, "name", "preferred_username", "azp"),
		Email:         firstString(claims, "email"),
		Organizations: aliases(claims[v.cfg.OrganizationClaim]),
	}
	// A client credentials token is a machine's (ADR-0029): Keycloak names the client in
	// client_id, and its service account's username starts with service-account-.
	if client := firstString(claims, "client_id"); client != "" {
		p.Kind, p.Client = ledger.KindMachine, client
	} else if strings.HasPrefix(firstString(claims, "preferred_username"), "service-account-") {
		p.Kind, p.Client = ledger.KindMachine, firstString(claims, "azp")
	}
	if v.cfg.StaffClaim != "" {
		for _, s := range stringsOf(claims[v.cfg.StaffClaim]) {
			if s == v.cfg.StaffValue {
				p.Staff = true
			}
		}
	}
	return p, nil
}

// firstString returns the first of the named claims that holds a non-empty string.
func firstString(claims map[string]json.RawMessage, names ...string) string {
	for _, n := range names {
		var s string
		if json.Unmarshal(claims[n], &s) == nil && s != "" {
			return s
		}
	}
	return ""
}

// stringsOf reads a claim holding a string or a list of strings.
func stringsOf(raw json.RawMessage) []string {
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var one string
	if json.Unmarshal(raw, &one) == nil && one != "" {
		return []string{one}
	}
	return nil
}

// aliases reads an organization claim: a list of aliases, one alias, or an object keyed by alias.
// A missing claim is no organization (Keycloak omits it for people in none).
func aliases(raw json.RawMessage) []string {
	if out := stringsOf(raw); out != nil {
		return out
	}
	var keyed map[string]json.RawMessage
	if json.Unmarshal(raw, &keyed) == nil {
		out := make([]string, 0, len(keyed))
		for alias := range keyed {
			out = append(out, alias)
		}
		return out
	}
	return nil
}

// BearerToken returns the token of an Authorization value "Bearer <token>", the scheme matched
// without regard to case (RFC 6750 section 2.1), and "" for anything else.
func BearerToken(authorization string) string {
	scheme, token, ok := strings.Cut(strings.TrimSpace(authorization), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
