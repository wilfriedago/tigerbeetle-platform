//go:build integration

package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc/oidctest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// testAuth is the in-process issuer a test's API trusts (ADR-0026), and the token of a person in
// the default organization, which every request of the tests sends unless it sends its own.
type testAuth struct {
	iss   *oidctest.Issuer
	token string
}

// trustTestIssuer starts an issuer and points cfg at it.
func trustTestIssuer(t *testing.T, cfg *config.Config) testAuth {
	t.Helper()
	iss := oidctest.New(t)
	cfg.AuthIssuerURL = iss.URL
	cfg.AuthAudience = oidctest.Audience
	cfg.AuthStaffClaim = "roles"
	cfg.AuthStaffValue = "tbp-staff"
	cfg.AuthOrganizationClaim = "organization"
	return testAuth{iss: iss, token: iss.Person(t, "tester", "default")}
}

// grant gives the subject a role in the organization with the alias, or on one of its ledgers,
// straight in the database: the API has no endpoint for grants until phase 5 of the plan.
func (a testAuth) grant(t *testing.T, postgresURL, subject, alias, ledgerName, role string) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var target *string
	if ledgerName != "" {
		target = &ledgerName
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO grants (organization_id, issuer, subject, ledger, role)
		SELECT id, $1, $2, $3, $4 FROM organizations WHERE alias = $5`, a.iss.URL, subject, target, role, alias)
	if err != nil {
		t.Fatalf("grant %s %s in %s: %v", role, subject, alias, err)
	}
}

// ownDefault makes the test person the owner of the default organization, so the tests of what
// a call does are not refused.
func (a testAuth) ownDefault(t *testing.T, postgresURL string) {
	t.Helper()
	a.grant(t, postgresURL, "tester", "default", "", "owner")
}

// withCredentials adds the test person's token and organization to every request that carries no
// Authorization header, so the tests of what a call does need not repeat them. Tests of
// authentication itself send their own header.
func (a testAuth) withCredentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", "Bearer "+a.token)
			if r.Header.Get("X-Organization") == "" {
				r.Header.Set("X-Organization", "default")
			}
		}
		next.ServeHTTP(w, r)
	})
}

// grpcCredentials sends the test person's token and organization with every unary call.
func (a testAuth) grpcCredentials() grpc.DialOption {
	return grpc.WithChainUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+a.token, "x-organization", "default")
		return invoker(ctx, method, req, reply, cc, opts...)
	})
}

// rpcHeaders are the broker headers of a call by the test person.
func (a testAuth) rpcHeaders(key string) rpc.Headers {
	return rpc.Headers{IdempotencyKey: key, Authorization: "Bearer " + a.token, Organization: "default"}
}

// TestAuthenticationEndToEnd checks that every transport refuses a call without a valid
// credential, the same way, and that the health probes stay open (ADR-0026, ADR-0032).
func TestAuthenticationEndToEnd(t *testing.T) {
	h := newHarness(t)
	h.expect(h.do(http.MethodPost, "/v1/ledgers/auth", "", map[string]any{}), http.StatusCreated, "create a ledger as the test person")

	t.Run("REST without a credential is 401 with a bearer challenge", func(t *testing.T) {
		code, header, raw := h.rawDo(http.MethodGet, "/v1/ledgers", "", map[string]string{"Authorization": "Basic dGVzdDp0ZXN0"})
		if code != http.StatusUnauthorized || header.Get("WWW-Authenticate") == "" || header.Get("X-Request-Id") == "" {
			t.Fatalf("status %d, headers %v, body %s", code, header, raw)
		}
		if !strings.Contains(string(raw), problem.TypeOf("unauthenticated")) {
			t.Fatalf("body %s, want the unauthenticated problem", raw)
		}
	})
	t.Run("an expired token is 401", func(t *testing.T) {
		expired := h.auth.iss.Token(t, map[string]any{"sub": "tester", "organization": []string{"default"}, "exp": time.Now().Add(-time.Hour).Unix()})
		code, _, raw := h.rawDo(http.MethodGet, "/v1/ledgers", "", map[string]string{"Authorization": "Bearer " + expired, "X-Organization": "default"})
		if code != http.StatusUnauthorized {
			t.Fatalf("status %d, body %s", code, raw)
		}
	})
	t.Run("the health probes need no credential", func(t *testing.T) {
		for _, path := range []string{"/healthz", "/readyz"} {
			code, _, raw := h.rawDo(http.MethodGet, path, "", map[string]string{"Authorization": "Basic dGVzdDp0ZXN0"})
			if code != http.StatusOK {
				t.Fatalf("%s: status %d, body %s", path, code, raw)
			}
		}
	})
	t.Run("an organization the person is not in is 404", func(t *testing.T) {
		token := h.auth.iss.Person(t, "outsider", "elsewhere")
		code, _, raw := h.rawDo(http.MethodGet, "/v1/ledgers/auth", "", map[string]string{"Authorization": "Bearer " + token, "X-Organization": "default"})
		if code != http.StatusNotFound || !strings.Contains(string(raw), "organization-not-found") {
			t.Fatalf("status %d, body %s", code, raw)
		}
	})
	t.Run("a ledger call that names no organization is 400", func(t *testing.T) {
		code, _, raw := h.rawDo(http.MethodGet, "/v1/ledgers/auth", "", map[string]string{"Authorization": "Bearer " + h.auth.token})
		if code != http.StatusBadRequest || !strings.Contains(string(raw), "organization-required") {
			t.Fatalf("status %d, body %s", code, raw)
		}
	})
	t.Run("GraphQL with a bad token is 401", func(t *testing.T) {
		code, _, raw := h.rawDo(http.MethodPost, "/graphql", `{"query":"{ ledgers { nodes { name } } }"}`,
			map[string]string{"Authorization": "Bearer nope", "Content-Type": "application/json"})
		if code != http.StatusUnauthorized {
			t.Fatalf("status %d, body %s", code, raw)
		}
	})
	t.Run("gRPC without a credential is Unauthenticated; health is open", func(t *testing.T) {
		conn := h.grpcConnWith()
		_, err := pb.NewLedgerServiceClient(conn).GetLedger(t.Context(), &pb.GetLedgerRequest{Ledger: "auth"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("GetLedger error = %v, want Unauthenticated", err)
		}
		if _, err := healthpb.NewHealthClient(conn).Check(t.Context(), &healthpb.HealthCheckRequest{}); err != nil {
			t.Fatalf("health check: %v", err)
		}
	})
}

// organization adds the platform's copy of an organization, as staff will through the API in
// phase 5 of the plan.
func organization(t *testing.T, postgresURL, alias string) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), postgresURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), `INSERT INTO organizations (id, alias, name) VALUES (gen_random_uuid(), $1, $1)`, alias); err != nil {
		t.Fatal(err)
	}
}

// TestAuthorizationEndToEnd checks the grants of ADR-0028 and the tenant isolation of ADR-0027
// through the transports: two organizations with a ledger of the same name, a viewer, an
// operator, a member without a role, and an Idempotency-Key used by two callers (ADR-0031).
func TestAuthorizationEndToEnd(t *testing.T) {
	h := newHarness(t)
	pg := h.cfgPostgresURL
	organization(t, pg, "globex")
	as := func(subject string, orgs ...string) string { return "Bearer " + h.auth.iss.Person(t, subject, orgs...) }
	owner, olga := "Bearer "+h.auth.token, as("olga", "default", "globex")
	viewer, operator := as("vic", "default"), as("opie", "default")
	send := func(authorization, org, method, path, body, key string) (int, string) {
		headers := map[string]string{"Authorization": authorization, "X-Organization": org, "Content-Type": "application/json"}
		if key != "" {
			headers["Idempotency-Key"] = key
		}
		code, _, raw := h.rawDo(method, path, body, headers)
		return code, string(raw)
	}
	payment := `{"postings":[{"source":"world","destination":"users:1:wallet","asset":"EUR/2","amount":"1250"}]}`

	if code, raw := send(owner, "default", http.MethodPost, "/v1/ledgers/payments", "{}", ""); code != http.StatusCreated {
		t.Fatalf("owner creates payments in default: %d %s", code, raw)
	}
	h.auth.grant(t, pg, "olga", "globex", "", "owner")
	if code, raw := send(olga, "globex", http.MethodPost, "/v1/ledgers/payments", `{"metadata":{"owner":"globex"}}`, ""); code != http.StatusCreated {
		t.Fatalf("olga creates payments in globex: %d %s", code, raw)
	}
	h.auth.grant(t, pg, "vic", "default", "payments", "viewer")
	h.auth.grant(t, pg, "opie", "default", "payments", "operator")

	t.Run("a member without a role sees no ledger", func(t *testing.T) {
		if code, raw := send(olga, "default", http.MethodGet, "/v1/ledgers/payments", "", ""); code != http.StatusNotFound {
			t.Fatalf("olga reads default's payments: %d %s", code, raw)
		}
		if code, raw := send(olga, "default", http.MethodGet, "/v1/ledgers", "", ""); code != http.StatusOK || strings.Contains(raw, "payments") {
			t.Fatalf("olga lists default's ledgers: %d %s", code, raw)
		}
	})
	t.Run("a viewer reads but may not write, on every transport", func(t *testing.T) {
		if code, raw := send(viewer, "default", http.MethodGet, "/v1/ledgers/payments", "", ""); code != http.StatusOK {
			t.Fatalf("viewer reads: %d %s", code, raw)
		}
		if code, raw := send(viewer, "default", http.MethodPost, "/v1/ledgers/payments/transactions", payment, "vic-1"); code != http.StatusForbidden || !strings.Contains(raw, problem.TypeOf("forbidden")) {
			t.Fatalf("viewer writes over REST: %d %s", code, raw)
		}
		code, raw := send(viewer, "default", http.MethodPost, "/graphql",
			`{"query":"mutation($input: CreateTransactionInput!) { createTransaction(input: $input) { id } }","variables":{"input":{"ledger":"payments","idempotencyKey":"vic-2","postings":[{"source":"world","destination":"users:1:wallet","asset":"EUR/2","amount":"1250"}]}}}`, "")
		if code != http.StatusOK || !strings.Contains(raw, `"FORBIDDEN"`) {
			t.Fatalf("viewer writes over GraphQL: %d %s", code, raw)
		}
		conn := h.grpcConnWith()
		ctx := metadata.AppendToOutgoingContext(t.Context(), "authorization", viewer, "x-organization", "default")
		_, err := pb.NewTransactionServiceClient(conn).CreateTransaction(ctx, &pb.CreateTransactionRequest{
			Ledger: "payments", IdempotencyKey: "vic-3",
			Postings: []*pb.Posting{{Source: "world", Destination: "users:1:wallet", Asset: "EUR/2", Amount: "1250"}},
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("viewer writes over gRPC: %v", err)
		}
		if code, raw := send(viewer, "default", http.MethodGet, "/v1/_reconciler", "", ""); code != http.StatusForbidden {
			t.Fatalf("viewer reads the reconciler: %d %s", code, raw)
		}
	})
	t.Run("an Idempotency-Key replays only for the caller that used it", func(t *testing.T) {
		if code, raw := send(operator, "default", http.MethodPost, "/v1/ledgers/payments/transactions", payment, "shared-key"); code != http.StatusCreated {
			t.Fatalf("operator writes: %d %s", code, raw)
		}
		if code, raw := send(operator, "default", http.MethodPost, "/v1/ledgers/payments/transactions", payment, "shared-key"); code != http.StatusCreated {
			t.Fatalf("operator replays: %d %s", code, raw)
		}
		code, raw := send(owner, "default", http.MethodPost, "/v1/ledgers/payments/transactions", payment, "shared-key")
		if code != http.StatusUnprocessableEntity || !strings.Contains(raw, "idempotency-key-reused") || strings.Contains(raw, `"id"`) {
			t.Fatalf("owner reuses the operator's key: %d %s", code, raw)
		}
	})
	t.Run("the other organization's ledger of the same name stays apart", func(t *testing.T) {
		code, raw := send(olga, "globex", http.MethodGet, "/v1/ledgers/payments", "", "")
		if code != http.StatusOK || !strings.Contains(raw, `"owner":"globex"`) {
			t.Fatalf("olga reads globex's payments: %d %s", code, raw)
		}
		code, raw = send(olga, "globex", http.MethodGet, "/v1/ledgers/payments/transactions", "", "")
		if code != http.StatusOK || strings.Contains(raw, `"postings"`) && strings.Contains(raw, "users:1:wallet") {
			t.Fatalf("olga sees default's transactions in globex: %d %s", code, raw)
		}
		if code, raw := send(owner, "globex", http.MethodGet, "/v1/ledgers/payments", "", ""); code != http.StatusNotFound {
			t.Fatalf("default's owner reads globex, where it is no member: %d %s", code, raw)
		}
	})
}
