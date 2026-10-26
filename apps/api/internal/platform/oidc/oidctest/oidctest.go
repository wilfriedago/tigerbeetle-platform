// Package oidctest is an in-process OpenID Connect issuer for tests (ADR-0025): an RSA key, an
// httptest server publishing the discovery document and the key set, and helpers that mint signed
// access tokens with any claims. Tests never need Keycloak.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Audience is the audience the tokens carry unless a test overrides the aud claim.
const Audience = "tbp-api"

// Issuer is a running test issuer.
type Issuer struct {
	URL    string
	key    *rsa.PrivateKey
	signer jose.Signer
}

// New starts an issuer that stops when the test ends.
func New(t testing.TB) *Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("at+jwt").WithHeader("kid", "test-key"))
	if err != nil {
		t.Fatal(err)
	}
	iss := &Issuer{key: key, signer: signer}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	iss.URL = srv.URL
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                iss.URL,
			"jwks_uri":                              iss.URL + "/keys",
			"authorization_endpoint":                iss.URL + "/auth",
			"token_endpoint":                        iss.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig"}}})
	})
	return iss
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Token signs the claims. iss, aud, iat and exp (five minutes) are filled in unless given.
func (i *Issuer) Token(t testing.TB, claims map[string]any) string {
	t.Helper()
	now := time.Now()
	all := map[string]any{"iss": i.URL, "aud": Audience, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix()}
	for k, v := range claims {
		all[k] = v
	}
	raw, err := jwt.Signed(i.signer).Claims(all).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Person returns a token for a person in the given organizations, by alias.
func (i *Issuer) Person(t testing.TB, subject string, organizations ...string) string {
	t.Helper()
	return i.Token(t, map[string]any{"sub": subject, "preferred_username": subject, "organization": organizations})
}

// Staff returns a token for a member of the platform's staff in no organization.
func (i *Issuer) Staff(t testing.TB, subject string) string {
	t.Helper()
	return i.Token(t, map[string]any{"sub": subject, "preferred_username": subject, "roles": []string{"tbp-staff"}})
}
