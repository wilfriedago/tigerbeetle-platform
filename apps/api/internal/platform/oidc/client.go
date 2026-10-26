package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// ClientConfig is a confidential client of the provider, such as the console's
// backend-for-frontend (ADR-0030).
type ClientConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL is where the provider sends the browser back with the code.
	RedirectURL string
	// Scopes are requested on top of openid.
	Scopes []string
}

// Tokens are what the provider issues to the client.
type Tokens struct {
	Access  string
	Refresh string
	// ID is the ID token, kept as the hint of the logout request.
	ID string
	// Expiry is when Access expires.
	Expiry time.Time
}

// Client runs the authorization code flow with PKCE and refreshes tokens.
type Client struct {
	ctx        context.Context
	oauth      oauth2.Config
	ids        *gooidc.IDTokenVerifier
	endSession string
}

// Client returns a client of the provider this verifier reads.
func (v *Verifier) Client(cfg ClientConfig) (*Client, error) {
	var claims struct {
		EndSession string `json:"end_session_endpoint"`
	}
	if err := v.provider.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc: discovery claims: %w", err)
	}
	return &Client{
		ctx: v.ctx,
		oauth: oauth2.Config{
			ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectURL,
			Endpoint: v.provider.Endpoint(), Scopes: append([]string{gooidc.ScopeOpenID}, cfg.Scopes...),
		},
		ids:        v.provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
		endSession: claims.EndSession,
	}, nil
}

// AuthCodeURL is where the browser signs in. state ties the answer to this request, nonce ties
// the ID token to it, and verifier is the PKCE secret whose hash the provider keeps (RFC 7636).
func (c *Client) AuthCodeURL(state, nonce, verifier string) string {
	return c.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), gooidc.Nonce(nonce))
}

// Exchange trades the code for tokens and checks the ID token and its nonce.
func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (Tokens, error) {
	tok, err := c.oauth.Exchange(c.with(ctx), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Tokens{}, fmt.Errorf("oidc: exchange: %w", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	id, err := c.ids.Verify(c.with(ctx), raw)
	if err != nil {
		return Tokens{}, fmt.Errorf("oidc: id token: %w", err)
	}
	if id.Nonce != nonce {
		return Tokens{}, errors.New("oidc: the id token answers another sign-in")
	}
	return Tokens{Access: tok.AccessToken, Refresh: tok.RefreshToken, ID: raw, Expiry: tok.Expiry}, nil
}

// Refresh trades a refresh token for new tokens. The provider may rotate the refresh token; the
// new one replaces it.
func (c *Client) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	src := c.oauth.TokenSource(c.with(ctx), &oauth2.Token{RefreshToken: refresh, Expiry: time.Unix(1, 0)})
	tok, err := src.Token()
	if err != nil {
		return Tokens{}, fmt.Errorf("oidc: refresh: %w", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	return Tokens{Access: tok.AccessToken, Refresh: tok.RefreshToken, ID: raw, Expiry: tok.Expiry}, nil
}

// EndSessionURL is where the browser ends its session at the provider (RP-initiated logout), or
// "" when the provider publishes no such endpoint.
func (c *Client) EndSessionURL(idTokenHint, postLogoutRedirect string) string {
	if c.endSession == "" {
		return ""
	}
	q := url.Values{"client_id": {c.oauth.ClientID}, "post_logout_redirect_uri": {postLogoutRedirect}}
	if idTokenHint != "" {
		q.Set("id_token_hint", idTokenHint)
	}
	return c.endSession + "?" + q.Encode()
}

// with carries the HTTP client the verifier was built with (Config.HTTPClient) into a call's
// context; go-oidc and x/oauth2 both read it from the oauth2.HTTPClient context key.
func (c *Client) with(ctx context.Context) context.Context {
	if client, ok := c.ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
		return context.WithValue(ctx, oauth2.HTTPClient, client)
	}
	return ctx
}
