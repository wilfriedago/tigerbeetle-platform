//go:build integration

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/bootstrap"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
)

// TestConsoleSignInEndToEnd runs the backend-for-frontend of ADR-0030 against the in-process
// issuer: sign-in, the session cookie on REST, the CSRF check, a refresh, and sign-out.
func TestConsoleSignInEndToEnd(t *testing.T) {
	var handler atomic.Value
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.Load().(http.Handler).ServeHTTP(w, r)
	}))
	base := "http://" + srv.Listener.Addr().String()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	cfg := config.Config{
		PostgresURL:           devstack.StartPostgres(t),
		TigerBeetleAddresses:  []string{devstack.StartTigerBeetle(t)},
		MigrateOnStart:        true,
		RESTEnabled:           true,
		GraphQLEnabled:        true,
		ReadinessTimeout:      2 * time.Second,
		TigerBeetleTimeout:    10 * time.Second,
		ReconcilerGracePeriod: time.Minute,
		ReconcilerInterval:    time.Hour,
		AuthClientID:          "tbp-console",
		AuthClientSecret:      "secret",
		AuthRedirectURL:       base + "/auth/callback",
		AuthConsoleOrigin:     base,
		AuthSessionKey:        base64.StdEncoding.EncodeToString(key),
		AuthCookieSecure:      false,
	}
	auth := trustTestIssuer(t, &cfg)
	app, err := bootstrap.Build(context.Background(), cfg, bootstrap.Service{Name: serviceName, Version: version}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(app.Close)
	handler.Store(app.Handler())
	srv.Start()
	t.Cleanup(srv.Close)
	auth.ownDefault(t, cfg.PostgresURL)
	auth.iss.SignInAs("tester", "default")

	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar, CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		// Follow the redirects of the sign-in, and stop at the console page it returns to.
		if strings.HasPrefix(req.URL.Path, "/default/") {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	call := func(method, path, body string, headers map[string]string) (int, string) {
		t.Helper()
		var r io.Reader
		if body != "" {
			r = strings.NewReader(body)
		}
		req, _ := http.NewRequest(method, base+path, r)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := browser.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusFound {
			return resp.StatusCode, resp.Header.Get("Location")
		}
		return resp.StatusCode, string(raw)
	}
	org := map[string]string{"X-Organization": "default"}

	if code, _ := call(http.MethodGet, "/v1/ledgers", "", org); code != http.StatusUnauthorized {
		t.Fatalf("before sign-in: %d, want 401", code)
	}
	code, location := call(http.MethodGet, "/auth/login?return_to=/default/ledgers", "", nil)
	if code != http.StatusFound || location != "/default/ledgers" {
		t.Fatalf("sign-in ended with %d %q, want a redirect to /default/ledgers", code, location)
	}
	site, _ := url.Parse(base)
	session := ""
	for _, c := range jar.Cookies(site) {
		if c.Name == "tbp_session" {
			session = c.Value
		}
	}
	if len(session) < 40 {
		t.Fatalf("no session cookie after sign-in: %v", jar.Cookies(site))
	}

	t.Run("the session answers who is signed in", func(t *testing.T) {
		code, raw := call(http.MethodGet, "/auth/session", "", org)
		var s struct {
			Subject       string
			Role          string
			Organizations []struct{ Alias string }
		}
		_ = json.Unmarshal([]byte(raw), &s)
		if code != http.StatusOK || s.Subject != "tester" || s.Role != "owner" || len(s.Organizations) != 1 || s.Organizations[0].Alias != "default" {
			t.Fatalf("session = %d %s", code, raw)
		}
	})
	t.Run("the cookie authenticates reads; writes need the console's origin", func(t *testing.T) {
		if code, raw := call(http.MethodGet, "/v1/ledgers", "", org); code != http.StatusOK {
			t.Fatalf("read with the cookie: %d %s", code, raw)
		}
		if code, raw := call(http.MethodPost, "/v1/ledgers/web", "{}", org); code != http.StatusForbidden || !strings.Contains(raw, "cross-site-request") {
			t.Fatalf("write without an origin: %d %s", code, raw)
		}
		if code, raw := call(http.MethodPost, "/v1/ledgers/web", "{}", map[string]string{"X-Organization": "default", "Origin": "https://evil.example"}); code != http.StatusForbidden {
			t.Fatalf("write from another origin: %d %s", code, raw)
		}
		if code, raw := call(http.MethodPost, "/v1/ledgers/web", "{}", map[string]string{"X-Organization": "default", "Sec-Fetch-Site": "same-origin"}); code != http.StatusCreated {
			t.Fatalf("write from the console: %d %s", code, raw)
		}
	})
	t.Run("an expiring access token is refreshed", func(t *testing.T) {
		pool, err := pgxpool.New(context.Background(), cfg.PostgresURL)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		if _, err := pool.Exec(context.Background(), `UPDATE sessions SET access_expires_at = now() - interval '1 minute'`); err != nil {
			t.Fatal(err)
		}
		if code, raw := call(http.MethodGet, "/v1/ledgers", "", org); code != http.StatusOK {
			t.Fatalf("read after the access token expired: %d %s", code, raw)
		}
		var expires time.Time
		if err := pool.QueryRow(context.Background(), `SELECT access_expires_at FROM sessions`).Scan(&expires); err != nil || !expires.After(time.Now()) {
			t.Fatalf("access_expires_at = %v, %v; want a refreshed expiry", expires, err)
		}
	})
	t.Run("sign-out ends the session", func(t *testing.T) {
		code, raw := call(http.MethodPost, "/auth/logout", "", map[string]string{"Origin": base})
		if code != http.StatusOK || !strings.Contains(raw, auth.iss.URL+"/logout?") {
			t.Fatalf("logout: %d %s", code, raw)
		}
		if code, _ := call(http.MethodGet, "/v1/ledgers", "", org); code != http.StatusUnauthorized {
			t.Fatalf("after sign-out: %d, want 401", code)
		}
	})
}
