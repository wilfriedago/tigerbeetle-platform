// Package oidctest is an in-process OpenID Connect issuer for tests (ADR-0026): an RSA key, an
// httptest server publishing the discovery document and the key set, and helpers that mint signed
// access tokens with any claims. Tests never need Keycloak.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
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

	mu sync.Mutex
	// signedIn is who the authorization endpoint signs in, without a login page.
	signedIn person
	codes    map[string]grant
	refresh  map[string]person
	serial   int
}

// person is who a token speaks for.
type person struct {
	subject       string
	organizations []string
}

// grant is what an authorization code stands for until it is exchanged.
type grant struct {
	person    person
	clientID  string
	nonce     string
	challenge string
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
	iss := &Issuer{key: key, signer: signer, codes: map[string]grant{}, refresh: map[string]person{}}
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
			"end_session_endpoint":                  iss.URL + "/logout",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("GET /auth", iss.authorize)
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) { iss.token(t, w, r) })
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

// Machine returns a client credentials token of the client, shaped as Keycloak issues one: the
// service account is the subject and client_id names the client (ADR-0029).
func (i *Issuer) Machine(t testing.TB, clientID, subject string) string {
	t.Helper()
	return i.Token(t, map[string]any{
		"sub": subject, "client_id": clientID, "azp": clientID, "preferred_username": "service-account-" + clientID,
	})
}

// SignInAs sets who the authorization endpoint signs in from now on: the browser of a test is
// redirected straight back with a code, as if this person had typed a password.
func (i *Issuer) SignInAs(subject string, organizations ...string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.signedIn = person{subject: subject, organizations: organizations}
}

// authorize answers the authorization endpoint (RFC 6749 section 4.1.1, with PKCE).
func (i *Issuer) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		http.Error(w, "the test issuer wants response_type=code with an S256 PKCE challenge", http.StatusBadRequest)
		return
	}
	i.mu.Lock()
	i.serial++
	code := fmt.Sprintf("code-%d", i.serial)
	i.codes[code] = grant{person: i.signedIn, clientID: q.Get("client_id"), nonce: q.Get("nonce"), challenge: q.Get("code_challenge")}
	i.mu.Unlock()
	back, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	v := back.Query()
	v.Set("code", code)
	v.Set("state", q.Get("state"))
	back.RawQuery = v.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound) //nolint:gosec // G710: a test issuer redirects where the test asks
}

// token answers the token endpoint: the authorization code grant, checked against the PKCE
// challenge, and the refresh token grant.
func (i *Issuer) token(t testing.TB, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	clientID := r.Form.Get("client_id")
	if id, _, ok := r.BasicAuth(); ok {
		clientID = id
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	var who person
	nonce := ""
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		g, ok := i.codes[r.Form.Get("code")]
		delete(i.codes, r.Form.Get("code"))
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || g.clientID != clientID || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
			writeTokenError(w)
			return
		}
		who, nonce = g.person, g.nonce
	case "refresh_token":
		p, ok := i.refresh[r.Form.Get("refresh_token")]
		if !ok {
			writeTokenError(w)
			return
		}
		delete(i.refresh, r.Form.Get("refresh_token"))
		who = p
	default:
		writeTokenError(w)
		return
	}
	i.serial++
	refresh := fmt.Sprintf("refresh-%d", i.serial)
	i.refresh[refresh] = who
	idClaims := map[string]any{"sub": who.subject, "aud": clientID}
	if nonce != "" {
		idClaims["nonce"] = nonce
	}
	writeJSON(w, map[string]any{
		"access_token":  i.Person(t, who.subject, who.organizations...),
		"id_token":      i.Token(t, idClaims),
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"expires_in":    300,
	})
}

func writeTokenError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
}

// Staff returns a token for a member of the platform's staff in no organization.
func (i *Issuer) Staff(t testing.TB, subject string) string {
	t.Helper()
	return i.Token(t, map[string]any{"sub": subject, "preferred_username": subject, "roles": []string{"tbp-staff"}})
}
