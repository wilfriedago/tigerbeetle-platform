//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

// TestOrganizationsAndMembersEndToEnd follows an organization from its creation by staff to its
// suspension (ADR-0027, ADR-0028), over REST and GraphQL, with an in-memory directory in place of
// Keycloak's admin API.
func TestOrganizationsAndMembersEndToEnd(t *testing.T) {
	h := newHarness(t)
	h.dir.AddPerson("bob", "bob@example.com")
	staff := "Bearer " + h.auth.iss.Staff(t, "sam")
	send := func(authorization, org, method, path, body string) (int, string) {
		t.Helper()
		headers := map[string]string{"Authorization": authorization, "Content-Type": "application/json"}
		if org != "" {
			headers["X-Organization"] = org
		}
		code, _, raw := h.rawDo(method, path, body, headers)
		return code, string(raw)
	}

	code, raw := send(staff, "", http.MethodPost, "/v1/organizations",
		`{"alias":"initech","name":"Initech","owner":{"email":"Ivy@Example.com","firstName":"Ivy"}}`)
	if code != http.StatusCreated || !strings.Contains(raw, `"alias":"initech"`) {
		t.Fatalf("staff create initech: %d %s", code, raw)
	}
	// Ivy had no account: the directory created one and sent the setup email.
	emails := h.dir.Emails()
	if len(emails) != 1 || !strings.HasPrefix(emails[0], "setup ") {
		t.Fatalf("emails = %v, want one account setup", emails)
	}
	ivySubject := strings.TrimPrefix(emails[0], "setup ")
	ivy := "Bearer " + h.auth.iss.Person(t, ivySubject, "initech")

	t.Run("staff manage organizations, nobody else does", func(t *testing.T) {
		if code, raw := send(staff, "", http.MethodPost, "/v1/organizations",
			`{"alias":"initech","name":"Again","owner":{"email":"other@example.com"}}`); code != http.StatusConflict {
			t.Fatalf("create initech twice: %d %s", code, raw)
		}
		if code, raw := send(ivy, "", http.MethodGet, "/v1/organizations", ""); code != http.StatusForbidden {
			t.Fatalf("an owner lists organizations: %d %s", code, raw)
		}
		code, raw := send(staff, "", http.MethodGet, "/v1/organizations", "")
		if code != http.StatusOK || !strings.Contains(raw, `"default"`) || !strings.Contains(raw, `"initech"`) {
			t.Fatalf("staff list organizations: %d %s", code, raw)
		}
		// Staff hold no role inside an organization.
		if code, raw := send(staff, "initech", http.MethodGet, "/v1/members", ""); code != http.StatusNotFound {
			t.Fatalf("staff read initech's members: %d %s", code, raw)
		}
	})

	t.Run("the owner invites and sets roles", func(t *testing.T) {
		code, raw := send(ivy, "initech", http.MethodPost, "/v1/members", `{"email":"bob@example.com","role":"admin"}`)
		if code != http.StatusCreated || !strings.Contains(raw, `"status":"invited"`) {
			t.Fatalf("invite bob: %d %s", code, raw)
		}
		if code, raw := send(ivy, "initech", http.MethodPost, "/v1/ledgers/payments", "{}"); code != http.StatusCreated {
			t.Fatalf("create payments: %d %s", code, raw)
		}
		if code, raw := send(ivy, "initech", http.MethodPut, "/v1/ledgers/payments/members/bob", `{"role":"operator"}`); code != http.StatusNoContent {
			t.Fatalf("bob operates payments: %d %s", code, raw)
		}
		if code, raw := send(ivy, "initech", http.MethodPut, "/v1/members/"+ivySubject+"/role", `{"role":"admin"}`); code != http.StatusConflict ||
			!strings.Contains(raw, problem.TypeOf("last-owner")) {
			t.Fatalf("the last owner steps down: %d %s", code, raw)
		}

		code, raw = send(ivy, "initech", http.MethodPost, "/graphql", `{"query":"{ members { email status role ledgers { ledger role } } }"}`)
		var got struct {
			Data struct {
				Members []struct {
					Email, Status string
					Role          *string
					Ledgers       []struct{ Ledger, Role string }
				}
			}
		}
		if err := json.Unmarshal([]byte(raw), &got); code != http.StatusOK || err != nil || len(got.Data.Members) != 2 {
			t.Fatalf("members over GraphQL: %d %s", code, raw)
		}
		for _, m := range got.Data.Members {
			switch m.Email {
			case "bob@example.com":
				if m.Status != "invited" || *m.Role != "admin" || len(m.Ledgers) != 1 || m.Ledgers[0].Role != "operator" {
					t.Errorf("bob = %+v", m)
				}
			case "ivy@example.com":
				if m.Status != "setup-pending" || *m.Role != "owner" {
					t.Errorf("ivy = %+v", m)
				}
			default:
				t.Errorf("unexpected member %+v", m)
			}
		}
	})

	t.Run("a member without org.members cannot manage", func(t *testing.T) {
		bob := "Bearer " + h.auth.iss.Person(t, "bob", "initech")
		h.auth.grant(t, h.cfgPostgresURL, "dee", "initech", "", "member")
		dee := "Bearer " + h.auth.iss.Person(t, "dee", "initech")
		if code, raw := send(dee, "initech", http.MethodGet, "/v1/members", ""); code != http.StatusForbidden {
			t.Fatalf("a member lists members: %d %s", code, raw)
		}
		// An admin may not make an owner.
		if code, raw := send(bob, "initech", http.MethodPut, "/v1/members/dee/role", `{"role":"owner"}`); code != http.StatusForbidden {
			t.Fatalf("an admin makes an owner: %d %s", code, raw)
		}
	})

	t.Run("a suspended organization answers 403", func(t *testing.T) {
		code, raw := send(staff, "", http.MethodPatch, "/v1/organizations/initech", `{"status":"suspended"}`)
		if code != http.StatusOK || !strings.Contains(raw, `"status":"suspended"`) {
			t.Fatalf("suspend: %d %s", code, raw)
		}
		code, raw = send(ivy, "initech", http.MethodGet, "/v1/members", "")
		if code != http.StatusForbidden || !strings.Contains(raw, problem.TypeOf("organization-suspended")) {
			t.Fatalf("a call in a suspended organization: %d %s", code, raw)
		}
		code, raw = send(staff, "", http.MethodPost, "/graphql",
			`{"query":"mutation { updateOrganization(input: {alias: \"initech\", status: \"active\", name: \"Initech Corp\"}) { name status } }"}`)
		if code != http.StatusOK || !strings.Contains(raw, `"name":"Initech Corp"`) || !strings.Contains(raw, `"status":"active"`) {
			t.Fatalf("reactivate over GraphQL: %d %s", code, raw)
		}
	})
}
