//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

// TestCredentialsEndToEnd creates an API key and a machine client, calls the API with each on
// REST, GraphQL and gRPC, and checks their roles, their organization and their revocation
// (ADR-0029).
func TestCredentialsEndToEnd(t *testing.T) {
	h := newHarness(t)
	owner := "Bearer " + h.auth.token
	send := func(authorization, org, method, path, body string) (int, string) {
		t.Helper()
		headers := map[string]string{"Authorization": authorization, "Content-Type": "application/json"}
		if org != "" {
			headers["X-Organization"] = org
		}
		code, _, raw := h.rawDo(method, path, body, headers)
		return code, string(raw)
	}
	if code, raw := send(owner, "default", http.MethodPost, "/v1/ledgers/payments", "{}"); code != http.StatusCreated {
		t.Fatalf("create payments: %d %s", code, raw)
	}
	if code, raw := send(owner, "default", http.MethodPost, "/v1/ledgers/payroll", "{}"); code != http.StatusCreated {
		t.Fatalf("create payroll: %d %s", code, raw)
	}

	code, raw := send(owner, "default", http.MethodPost, "/v1/api-keys", `{"name":"payouts","ledgers":[{"ledger":"payments","role":"viewer"}]}`)
	var created struct {
		Key    struct{ ID, Status string }
		Secret string
	}
	if err := json.Unmarshal([]byte(raw), &created); code != http.StatusCreated || err != nil || !strings.HasPrefix(created.Secret, "tbp_"+created.Key.ID+"_") {
		t.Fatalf("create key: %d %s", code, raw)
	}
	key := "Bearer " + created.Secret
	ledgers := pb.NewLedgerServiceClient(h.grpcConnWith())

	t.Run("an API key acts with its roles, in its organization", func(t *testing.T) {
		if code, raw := send(key, "", http.MethodGet, "/v1/ledgers/payments", ""); code != http.StatusOK {
			t.Fatalf("key reads payments: %d %s", code, raw)
		}
		if code, raw := send(key, "", http.MethodGet, "/v1/ledgers/payroll", ""); code != http.StatusNotFound {
			t.Fatalf("key reads payroll, which it has no role on: %d %s", code, raw)
		}
		code, _, write := h.rawDo(http.MethodPost, "/v1/ledgers/payments/transactions",
			`{"postings":[{"source":"world","destination":"a:1","asset":"EUR/2","amount":"1"}]}`,
			map[string]string{"Authorization": key, "Content-Type": "application/json", "Idempotency-Key": "key-write-1"})
		if code != http.StatusForbidden {
			t.Fatalf("a viewer key writes: %d %s", code, write)
		}
		if code, raw := send(key, "acme", http.MethodGet, "/v1/ledgers/payments", ""); code != http.StatusForbidden || !strings.Contains(raw, "belongs to the organization default") {
			t.Fatalf("key names another organization: %d %s", code, raw)
		}
		code, raw := send(key, "", http.MethodPost, "/graphql", `{"query":"{ ledger(name: \"payments\") { name } }"}`)
		if code != http.StatusOK || !strings.Contains(raw, `"name":"payments"`) {
			t.Fatalf("key over GraphQL: %d %s", code, raw)
		}
		ctx := metadata.AppendToOutgoingContext(t.Context(), "authorization", key)
		if _, err := ledgers.GetLedger(ctx, &pb.GetLedgerRequest{Ledger: "payments"}); err != nil {
			t.Fatalf("key over gRPC: %v", err)
		}
		if code, raw := send("Bearer tbp_"+created.Key.ID+"_wrongsecret", "", http.MethodGet, "/v1/ledgers/payments", ""); code != http.StatusUnauthorized {
			t.Fatalf("a wrong secret: %d %s", code, raw)
		}
	})

	code, raw = send(owner, "default", http.MethodPost, "/v1/machine-clients", `{"name":"recon","ledgers":[{"ledger":"payroll","role":"operator"}]}`)
	var client struct {
		Client   struct{ ClientID, Subject string }
		Secret   string
		TokenURL string `json:"tokenUrl"`
	}
	if err := json.Unmarshal([]byte(raw), &client); code != http.StatusCreated || err != nil || client.Client.ClientID != "default-recon" || client.Secret == "" {
		t.Fatalf("create machine client: %d %s", code, raw)
	}
	machine := "Bearer " + h.auth.iss.Machine(t, client.Client.ClientID, client.Client.Subject)

	t.Run("a machine client's tokens act for its organization", func(t *testing.T) {
		if code, raw := send(machine, "", http.MethodGet, "/v1/ledgers/payroll", ""); code != http.StatusOK {
			t.Fatalf("machine reads payroll: %d %s", code, raw)
		}
		if code, raw := send(machine, "", http.MethodGet, "/v1/ledgers/payments", ""); code != http.StatusNotFound {
			t.Fatalf("machine reads payments: %d %s", code, raw)
		}
		stranger := "Bearer " + h.auth.iss.Machine(t, "someone-else", "service-account-someone-else")
		if code, raw := send(stranger, "", http.MethodGet, "/v1/ledgers/payroll", ""); code != http.StatusUnauthorized {
			t.Fatalf("a client the platform did not create: %d %s", code, raw)
		}
		code, raw := send(owner, "default", http.MethodGet, "/v1/machine-clients/default-recon", "")
		if code != http.StatusOK || !strings.Contains(raw, `"lastTokenAt"`) {
			t.Fatalf("the client's last token: %d %s", code, raw)
		}
	})

	t.Run("revoked and deleted credentials stop at once", func(t *testing.T) {
		if code, raw := send(owner, "default", http.MethodDelete, "/v1/api-keys/"+created.Key.ID, ""); code != http.StatusNoContent {
			t.Fatalf("revoke key: %d %s", code, raw)
		}
		code, raw := send(key, "", http.MethodGet, "/v1/ledgers/payments", "")
		if code != http.StatusUnauthorized || !strings.Contains(raw, problem.TypeOf("unauthenticated")) {
			t.Fatalf("a revoked key: %d %s", code, raw)
		}
		ctx := metadata.AppendToOutgoingContext(t.Context(), "authorization", key)
		_, err := ledgers.GetLedger(ctx, &pb.GetLedgerRequest{Ledger: "payments"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("a revoked key over gRPC: %v", err)
		}
		if code, raw := send(owner, "default", http.MethodDelete, "/v1/machine-clients/default-recon", ""); code != http.StatusNoContent {
			t.Fatalf("delete client: %d %s", code, raw)
		}
		if h.dir.HasClient("default-recon") {
			t.Error("the Keycloak client was not deleted")
		}
		if code, raw := send(machine, "", http.MethodGet, "/v1/ledgers/payroll", ""); code != http.StatusUnauthorized {
			t.Fatalf("a deleted client's token: %d %s", code, raw)
		}
	})

	t.Run("only org.credentials manages credentials", func(t *testing.T) {
		h.auth.grant(t, h.cfgPostgresURL, "mo", "default", "", "member")
		member := "Bearer " + h.auth.iss.Person(t, "mo", "default")
		if code, raw := send(member, "default", http.MethodGet, "/v1/api-keys", ""); code != http.StatusForbidden {
			t.Fatalf("a member lists keys: %d %s", code, raw)
		}
	})
}
