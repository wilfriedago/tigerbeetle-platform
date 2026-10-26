package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// request is what the fake Keycloak received.
type request struct {
	Method, Path, Query, ContentType, Body string
}

// fakeKeycloak answers the token endpoint and records every admin call, answering each with the
// next canned answer for its method and path.
type fakeKeycloak struct {
	mu       sync.Mutex
	received []request
	answers  map[string]answer
	tokens   int
}

type answer struct {
	status   int
	body     string
	location string
}

func newFake(t *testing.T, answers map[string]answer) (*fakeKeycloak, *Admin) {
	t.Helper()
	f := &fakeKeycloak{answers: answers}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	d, err := New(Config{
		RealmURL: srv.URL + "/realms/tbp",
		ClientID: "tbp-admin", ClientSecret: "secret", SetupClientID: "tbp-console",
	})
	if err != nil {
		t.Fatal(err)
	}
	return f, d
}

func (f *fakeKeycloak) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/realms/tbp/protocol/openid-connect/token" {
		f.tokens++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"admin-token","token_type":"Bearer","expires_in":300}`)
		return
	}
	if r.Header.Get("Authorization") != "Bearer admin-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	body, _ := io.ReadAll(r.Body)
	f.received = append(f.received, request{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Content-Type"), string(body)})
	a, ok := f.answers[r.Method+" "+r.URL.Path]
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if a.location != "" {
		w.Header().Set("Location", a.location)
	}
	w.WriteHeader(a.status)
	_, _ = io.WriteString(w, a.body)
}

func TestNewDerivesTheAdminURL(t *testing.T) {
	cases := []struct {
		realm, admin string
		ok           bool
	}{
		{"http://keycloak:8080/realms/tbp", "http://keycloak:8080/admin/realms/tbp", true},
		{"https://id.example/auth/realms/tbp/", "https://id.example/auth/admin/realms/tbp", true},
		{"http://keycloak:8080/tbp", "", false},
		{"keycloak/realms/tbp", "", false},
	}
	for _, c := range cases {
		d, err := New(Config{RealmURL: c.realm, ClientID: "id", ClientSecret: "secret"})
		if (err == nil) != c.ok {
			t.Fatalf("New(%q) error = %v, want ok %v", c.realm, err, c.ok)
		}
		if c.ok && d.admin != c.admin {
			t.Errorf("New(%q) admin = %q, want %q", c.realm, d.admin, c.admin)
		}
	}
}

func TestOrganizations(t *testing.T) {
	f, d := newFake(t, map[string]answer{
		"GET /admin/realms/tbp/organizations":     {200, `[{"id":"o-2","alias":"acme-eu","name":"Acme EU"},{"id":"o-1","alias":"acme","name":"Acme"}]`, ""},
		"POST /admin/realms/tbp/organizations":    {201, "", "http://kc/admin/realms/tbp/organizations/o-9"},
		"GET /admin/realms/tbp/organizations/o-1": {200, `{"id":"o-1","alias":"acme","name":"Acme","enabled":true,"domains":[{"name":"acme.test"}]}`, ""},
	})
	ctx := context.Background()

	id, err := d.OrganizationID(ctx, "acme")
	if err != nil || id != "o-1" {
		t.Fatalf("OrganizationID = %q, %v; want o-1", id, err)
	}
	if _, err := d.OrganizationID(ctx, "globex"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("OrganizationID(globex) error = %v, want ErrNotFound", err)
	}
	id, err = d.CreateOrganization(ctx, "initech", "Initech")
	if err != nil || id != "o-9" {
		t.Fatalf("CreateOrganization = %q, %v; want o-9", id, err)
	}
	if err := d.RenameOrganization(ctx, "o-1", "acme", "Acme Corp"); err != nil {
		t.Fatal(err)
	}

	want := []request{
		{"GET", "/admin/realms/tbp/organizations", "briefRepresentation=true&q=alias%3Aacme", "", ""},
		{"GET", "/admin/realms/tbp/organizations", "briefRepresentation=true&q=alias%3Aglobex", "", ""},
		{"POST", "/admin/realms/tbp/organizations", "", "application/json", `{"name":"Initech","alias":"initech","enabled":true}`},
		{"GET", "/admin/realms/tbp/organizations/o-1", "", "", ""},
		{
			"PUT", "/admin/realms/tbp/organizations/o-1", "", "application/json",
			`{"alias":"acme","domains":[{"name":"acme.test"}],"enabled":true,"id":"o-1","name":"Acme Corp"}`,
		},
	}
	if diff := cmp.Diff(want, f.received); diff != "" {
		t.Errorf("requests (-want +got):\n%s", diff)
	}
	if f.tokens != 1 {
		t.Errorf("token requests = %d, want 1: the token is reused", f.tokens)
	}
}

func TestPeople(t *testing.T) {
	f, d := newFake(t, map[string]answer{
		"GET /admin/realms/tbp/users":                         {200, `[{"id":"u-1","email":"ada@example.com","firstName":"Ada","requiredActions":[]}]`, ""},
		"POST /admin/realms/tbp/users":                        {201, "", "http://kc/admin/realms/tbp/users/u-2"},
		"GET /admin/realms/tbp/organizations/o-1/members/u-3": {404, `{"error":"HTTP 404 Not Found"}`, ""},
	})
	ctx := context.Background()

	u, err := d.UserByEmail(ctx, "Ada@Example.com")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(User{ID: "u-1", Email: "ada@example.com", FirstName: "Ada", RequiredActions: []string{}}, u); diff != "" {
		t.Errorf("UserByEmail (-want +got):\n%s", diff)
	}
	u, err = d.CreateUser(ctx, "new@example.com", "New", "")
	if err != nil {
		t.Fatal(err)
	}
	if !u.SetupPending() || u.ID != "u-2" {
		t.Errorf("CreateUser = %+v, want u-2 with setup pending", u)
	}
	if err := d.SendAccountSetup(ctx, "u-2"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddMember(ctx, "o-1", "u-2"); err != nil {
		t.Fatal(err)
	}
	if err := d.Invite(ctx, "o-1", "u-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Member(ctx, "o-1", "u-3"); !errors.Is(err, ledger.ErrNotFound) {
		t.Errorf("Member of a non-member error = %v, want ErrNotFound", err)
	}

	want := []request{
		{"GET", "/admin/realms/tbp/users", "briefRepresentation=false&email=Ada%40Example.com&exact=true", "", ""},
		{
			"POST", "/admin/realms/tbp/users", "", "application/json",
			`{"username":"new@example.com","email":"new@example.com","firstName":"New","enabled":true,"emailVerified":false,"requiredActions":["VERIFY_EMAIL","UPDATE_PASSWORD"]}`,
		},
		{"PUT", "/admin/realms/tbp/users/u-2/execute-actions-email", "client_id=tbp-console&lifespan=604800", "application/json", `["VERIFY_EMAIL","UPDATE_PASSWORD"]`},
		{"POST", "/admin/realms/tbp/organizations/o-1/members", "", "application/json", `"u-2"`},
		{"POST", "/admin/realms/tbp/organizations/o-1/members/invite-existing-user", "", "application/x-www-form-urlencoded", "id=u-1"},
		{"GET", "/admin/realms/tbp/organizations/o-1/members/u-3", "", "", ""},
	}
	if diff := cmp.Diff(want, f.received); diff != "" {
		t.Errorf("requests (-want +got):\n%s", diff)
	}
}

func TestMembersReadsEveryPage(t *testing.T) {
	page := make([]User, pageSize)
	for i := range page {
		page[i] = User{ID: "u", Email: "p@example.com"}
	}
	full, _ := json.Marshal(page)
	f := &pagedFake{pages: []string{string(full), `[{"id":"last","email":"z@example.com","requiredActions":["UPDATE_PASSWORD"]}]`}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	d, err := New(Config{RealmURL: srv.URL + "/realms/tbp", ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	users, err := d.Members(context.Background(), "o-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != pageSize+1 || !users[pageSize].SetupPending() {
		t.Fatalf("Members = %d users, last %+v; want %d, the last with setup pending", len(users), users[len(users)-1], pageSize+1)
	}
	if diff := cmp.Diff([]string{"briefRepresentation=false&first=0&max=100", "briefRepresentation=false&first=100&max=100"}, f.queries); diff != "" {
		t.Errorf("queries (-want +got):\n%s", diff)
	}
}

type pagedFake struct {
	pages   []string
	queries []string
}

func (f *pagedFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/token") {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"t","token_type":"Bearer","expires_in":300}`)
		return
	}
	f.queries = append(f.queries, r.URL.RawQuery)
	_, _ = io.WriteString(w, f.pages[len(f.queries)-1])
}

func TestStatusErrors(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusNotFound, ledger.ErrNotFound},
		{http.StatusConflict, ledger.ErrAlreadyExists},
		{http.StatusBadGateway, ledger.ErrUnavailable},
		{http.StatusForbidden, nil},
	}
	for _, c := range cases {
		_, d := newFake(t, map[string]answer{"POST /admin/realms/tbp/organizations": {c.status, `{"errorMessage":"secret detail"}`, ""}})
		_, err := d.CreateOrganization(context.Background(), "acme", "Acme")
		if err == nil {
			t.Fatalf("status %d: no error", c.status)
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("status %d: error %v, want %v", c.status, err, c.want)
		}
		if strings.Contains(err.Error(), "secret detail") {
			t.Errorf("status %d: error %q carries Keycloak's message", c.status, err)
		}
	}
}
