package oidc_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc/oidctest"
)

func TestClientFlow(t *testing.T) {
	iss := oidctest.New(t)
	iss.SignInAs("ada", "acme")
	v := newVerifier(t, iss)
	client, err := v.Client(oidc.ClientConfig{ClientID: "tbp-console", ClientSecret: "s", RedirectURL: "http://console.test/api/auth/callback", Scopes: []string{"organization:*"}})
	if err != nil {
		t.Fatal(err)
	}
	verifier := "a-pkce-verifier-of-at-least-forty-three-characters-long"
	authURL := client.AuthCodeURL("state-1", "nonce-1", verifier)
	if !strings.Contains(authURL, "code_challenge_method=S256") || !strings.Contains(authURL, "organization%3A%2A") {
		t.Fatalf("authorization URL %s lacks PKCE or the scope", authURL)
	}
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	back, _ := url.Parse(resp.Header.Get("Location"))
	if back.Query().Get("state") != "state-1" {
		t.Fatalf("state = %q", back.Query().Get("state"))
	}
	code := back.Query().Get("code")

	if _, err := client.Exchange(context.Background(), code, "another-verifier-that-does-not-match-the-challenge", "nonce-1"); err == nil {
		t.Fatal("a code was exchanged with the wrong PKCE verifier")
	}
	resp, _ = noRedirect.Get(authURL)
	_ = resp.Body.Close()
	back, _ = url.Parse(resp.Header.Get("Location"))
	if _, err := client.Exchange(context.Background(), back.Query().Get("code"), verifier, "another-nonce"); err == nil {
		t.Fatal("an ID token of another sign-in was accepted")
	}
	resp, _ = noRedirect.Get(authURL)
	_ = resp.Body.Close()
	back, _ = url.Parse(resp.Header.Get("Location"))
	tokens, err := client.Exchange(context.Background(), back.Query().Get("code"), verifier, "nonce-1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	p, err := v.Verify(context.Background(), tokens.Access)
	if err != nil || p.Subject != "ada" || !p.MemberOf("acme") {
		t.Fatalf("access token principal = %+v, %v", p, err)
	}
	refreshed, err := client.Refresh(context.Background(), tokens.Refresh)
	if err != nil || refreshed.Access == "" || refreshed.Refresh == tokens.Refresh {
		t.Fatalf("Refresh = %+v, %v; want a new access and a rotated refresh token", refreshed, err)
	}
	if _, err := client.Refresh(context.Background(), tokens.Refresh); err == nil {
		t.Fatal("a rotated refresh token was accepted again")
	}
	if logout := client.EndSessionURL(tokens.ID, "http://console.test/"); !strings.HasPrefix(logout, iss.URL+"/logout?") || !strings.Contains(logout, "id_token_hint=") {
		t.Fatalf("end session URL = %s", logout)
	}
}
