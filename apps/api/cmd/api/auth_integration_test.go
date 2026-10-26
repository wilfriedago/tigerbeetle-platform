//go:build integration

package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/oidc/oidctest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// testAuth is the in-process issuer a test's API trusts (ADR-0025), and the token of a person in
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
// credential, the same way, and that the health probes stay open (ADR-0025, ADR-0031).
func TestAuthenticationEndToEnd(t *testing.T) {
	h := newHarness(t)
	h.expect(h.do(http.MethodPost, "/v1/ledgers/auth", "", map[string]any{}), http.StatusCreated, "create a ledger as the test person")

	t.Run("REST without a credential is 401 with a bearer challenge", func(t *testing.T) {
		code, header, raw := h.rawDo(http.MethodGet, "/v1/ledgers", "", map[string]string{"Authorization": "Basic dGVzdDp0ZXN0"})
		if code != http.StatusUnauthorized || header.Get("WWW-Authenticate") == "" || header.Get("X-Request-Id") == "" {
			t.Fatalf("status %d, headers %v, body %s", code, header, raw)
		}
		if !strings.Contains(string(raw), "/problems/unauthenticated") {
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
