// Package web serves the console's sign-in through the backend-for-frontend (ADR-0030): browser
// redirects and cookies, not API operations, so they are not in the REST spec.
//
//	GET  /auth/login?return_to=/acme/ledgers   start a sign-in at the identity provider
//	GET  /auth/callback                        finish it and set the session cookie
//	POST /auth/logout                          end the session; answers where to end it at the provider
//	GET  /auth/session                         the signed-in person, their organizations and roles
//
// The console reaches them through its own origin (/api/auth/...), so the cookies are first-party.
package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// Config are the cookie and redirect settings.
type Config struct {
	// Secure marks the cookies Secure and gives them the __Host- prefix; false only for
	// plain-HTTP development.
	Secure bool
	// ConsoleOrigin is where the console runs, such as http://localhost:5173.
	ConsoleOrigin string
	// AccountURL is the identity provider's page where people manage their own account:
	// password, two-factor, sessions. The console links to it.
	AccountURL string
}

// SessionCookie is the name of the session cookie.
func (c Config) SessionCookie() string { return c.cookie("tbp_session") }

func (c Config) signInCookie() string { return c.cookie("tbp_signin") }

// cookie names a cookie. The __Host- prefix makes the browser refuse it unless it is Secure, has
// Path=/ and no Domain, so no other host can set or read it.
func (c Config) cookie(name string) string {
	if c.Secure {
		return "__Host-" + name
	}
	return name
}

// Paths that need no session: they start, finish and end one.
var Paths = []string{"/auth/login", "/auth/callback", "/auth/logout"}

type handlers struct {
	sessions application.Sessions
	viewer   application.ViewerQueries
	cfg      Config
}

// Register adds the routes to mux.
func Register(mux *http.ServeMux, sessions application.Sessions, viewer application.ViewerQueries, cfg Config) {
	h := handlers{sessions: sessions, viewer: viewer, cfg: cfg}
	mux.HandleFunc("GET /auth/login", h.login)
	mux.HandleFunc("GET /auth/callback", h.callback)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("GET /auth/session", h.session)
}

func (h handlers) setCookie(w http.ResponseWriter, name, value string, maxAge int) {
	// Secure is configurable for plain-HTTP development only (AUTH_COOKIE_SECURE).
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: HttpOnly and SameSite are always set; see above
		Name: name, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: h.cfg.Secure, SameSite: http.SameSiteLaxMode,
	})
}

// returnTo keeps only a path of the console, never another site: "/acme/ledgers" stays, while
// "//evil.example" and "https://evil.example" become "/".
func returnTo(raw string) string {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return "/"
	}
	return raw
}

func (h handlers) login(w http.ResponseWriter, r *http.Request) {
	authURL, cookie, err := h.sessions.Begin(r.Context(), returnTo(r.URL.Query().Get("return_to")))
	if err != nil {
		restapi.WriteProblem(w, r, restapi.Fail(r.Context(), err).Problem)
		return
	}
	h.setCookie(w, h.cfg.signInCookie(), cookie, 600)
	http.Redirect(w, r, authURL, http.StatusFound) //nolint:gosec // G710: the provider's own authorization endpoint
}

func (h handlers) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cookie, err := r.Cookie(h.cfg.signInCookie())
	if err != nil || q.Get("error") != "" {
		restapi.WriteProblem(w, r, problem.From(ledger.NewProblem(ledger.ErrUnauthenticated, "sign-in-failed",
			"the sign-in could not be completed; start again")))
		return
	}
	h.setCookie(w, h.cfg.signInCookie(), "", -1)
	id, to, err := h.sessions.Complete(r.Context(), cookie.Value, q.Get("code"), q.Get("state"))
	if err != nil {
		logging.From(r.Context()).InfoContext(r.Context(), "sign-in failed", slog.Any("error", err))
		restapi.WriteProblem(w, r, restapi.Fail(r.Context(), err).Problem)
		return
	}
	h.setCookie(w, h.cfg.SessionCookie(), id, int(application.SessionLifetime.Seconds()))
	http.Redirect(w, r, to, http.StatusFound) //nolint:gosec // G710: returnTo keeps only paths of the console
}

func (h handlers) logout(w http.ResponseWriter, r *http.Request) {
	if !restapi.SameOrigin(r, h.cfg.ConsoleOrigin) {
		restapi.WriteProblem(w, r, problem.From(ledger.NewProblem(ledger.ErrForbidden, "cross-site-request",
			"a request that changes state must come from the console")))
		return
	}
	logoutURL := ""
	if c, err := r.Cookie(h.cfg.SessionCookie()); err == nil && c.Value != "" {
		u, err := h.sessions.End(r.Context(), c.Value, h.cfg.ConsoleOrigin+"/")
		if err != nil {
			restapi.WriteProblem(w, r, restapi.Fail(r.Context(), err).Problem)
			return
		}
		logoutURL = u
	}
	h.setCookie(w, h.cfg.SessionCookie(), "", -1)
	writeJSON(w, map[string]string{"logoutUrl": logoutURL})
}

// sessionView is the answer of GET /auth/session.
type sessionView struct {
	Subject       string                 `json:"subject"`
	Name          string                 `json:"name"`
	Email         string                 `json:"email,omitempty"`
	AccountURL    string                 `json:"accountUrl,omitempty"`
	Staff         bool                   `json:"staff"`
	Organizations []organizationView     `json:"organizations"`
	Role          ledger.Role            `json:"role,omitempty"`
	Ledgers       map[string]ledger.Role `json:"ledgers"`
}

type organizationView struct {
	Alias            string                 `json:"alias"`
	Name             string                 `json:"name"`
	Status           string                 `json:"status"`
	SuspensionReason string                 `json:"suspensionReason,omitempty"`
	Role             ledger.Role            `json:"role,omitempty"`
	Ledgers          map[string]ledger.Role `json:"ledgers"`
}

// session answers who is signed in. The middleware has already authenticated the request with
// the session cookie; with X-Organization it also gives the roles in that organization.
func (h handlers) session(w http.ResponseWriter, r *http.Request) {
	v, err := h.viewer.Viewer(r.Context())
	if err != nil {
		restapi.WriteProblem(w, r, restapi.Fail(r.Context(), err).Problem)
		return
	}
	out := sessionView{Subject: v.Subject, Name: v.Name, Email: v.Email, AccountURL: h.cfg.AccountURL, Staff: v.Staff, Role: v.Role, Ledgers: v.Ledgers, Organizations: []organizationView{}}
	for _, o := range v.Organizations {
		out.Organizations = append(out.Organizations, organizationView{
			Alias: o.Alias, Name: o.Name, Status: string(o.Status), SuspensionReason: o.SuspensionReason, Role: o.Role, Ledgers: o.Ledgers,
		})
	}
	writeJSON(w, out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
