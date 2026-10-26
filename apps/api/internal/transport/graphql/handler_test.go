package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/readiness"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// serve runs one GraphQL request through the production middleware.
func serve(t *testing.T, m Modules, body string) map[string]any {
	t.Helper()
	rec := post(m, body)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("status %d, body %q: %v", rec.Code, rec.Body.String(), err)
	}
	return out
}

// post sends body to POST /graphql through the production middleware.
func post(m Modules, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.Handle("POST /graphql", NewHandler(NewResolver(m)))
	h := restapi.Handler(mux, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "req-1")
	h.ServeHTTP(rec, req)
	return rec
}

func healthModules() Modules {
	checker := readiness.New(50 * time.Millisecond)
	checker.Require(healthdto.ProbePostgres, func(context.Context) error { return nil })
	checker.Require(healthdto.ProbeTigerBeetle, func(context.Context) error { return fmt.Errorf("dial: secret driver text") })
	return Modules{Health: healthdto.Source{Readiness: checker, Service: healthdto.Service{Name: "svc", Version: "1.2.3"}}}
}

func TestServiceInfo(t *testing.T) {
	t.Parallel()
	out := serve(t, healthModules(), `{"query":"{ serviceInfo { service version dependencies { postgres { status } tigerbeetle { status error } } } }"}`)
	want := map[string]any{"serviceInfo": map[string]any{
		"service": "svc", "version": "1.2.3",
		"dependencies": map[string]any{
			"postgres":    map[string]any{"status": "up"},
			"tigerbeetle": map[string]any{"status": "down", "error": "unreachable"},
		},
	}}
	if diff := cmp.Diff(want, out["data"]); diff != "" {
		t.Fatalf("data (-want +got):\n%s", diff)
	}
}

func TestGraphQLOwnErrorsKeepTheirCode(t *testing.T) {
	t.Parallel()
	out := serve(t, healthModules(), `{"query":"{ serviceInfo { nope } }"}`)
	errs, _ := out["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("errors = %v", out)
	}
	ext := errs[0].(map[string]any)["extensions"].(map[string]any)
	if ext["code"] != "GRAPHQL_VALIDATION_FAILED" || ext["type"] != problem.TypeOf("invalid-request") ||
		ext["title"] != "Invalid request" || ext["status"] != float64(400) || ext["requestId"] != "req-1" {
		t.Fatalf("extensions = %v", ext)
	}
}

// TestBodiesThatAreNoGraphQLRequest checks that a body gqlgen cannot even read as a request is
// answered with a problem, without quoting the body back.
func TestBodiesThatAreNoGraphQLRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantType   string
	}{
		{name: "not JSON", body: `query=secret`, wantStatus: 400, wantType: problem.TypeOf("invalid-request")},
		{name: "a JSON array", body: `["secret"]`, wantStatus: 400, wantType: problem.TypeOf("invalid-request")},
		{name: "too large", body: `{"query":"` + strings.Repeat("a", maxBodyBytes) + `"}`, wantStatus: 413, wantType: problem.TypeBlank},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := post(healthModules(), tt.body)
			if rec.Code != tt.wantStatus || rec.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("status %d, Content-Type %q, want %d problem+json", rec.Code, rec.Header().Get("Content-Type"), tt.wantStatus)
			}
			var p struct {
				Type      string `json:"type"`
				Status    int    `json:"status"`
				RequestID string `json:"requestId"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Type != tt.wantType || p.Status != tt.wantStatus || p.RequestID != "req-1" {
				t.Fatalf("problem %+v (%v), want type %q", p, err, tt.wantType)
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Errorf("the body was quoted back: %s", rec.Body.String())
			}
		})
	}
}

func TestDepthLimit(t *testing.T) {
	t.Parallel()
	if depth := depthOf(t, "{ serviceInfo { dependencies { postgres { status } } } }"); depth != 4 {
		t.Fatalf("depth = %d, want 4", depth)
	}
	nested := "{ a" + strings.Repeat(" { a", maxDepth) + strings.Repeat(" }", maxDepth) + " }"
	doc, err := parseQuery(nested)
	if err != nil {
		t.Fatal(err)
	}
	op := &graphql.OperationContext{Operation: doc.Operations[0], Doc: doc}
	if err := (depthLimit{max: maxDepth}).MutateOperationContext(context.Background(), op); err == nil || err.Extensions["code"] != "DEPTH_LIMIT_EXCEEDED" {
		t.Fatalf("an operation %d levels deep was not refused: %v", maxDepth+1, err)
	}
	op.Doc, op.Operation = mustParse(t, "{ a { b } }"), nil
	if err := (depthLimit{max: maxDepth}).MutateOperationContext(context.Background(), op); err != nil {
		t.Fatalf("an operation without a selection was refused: %v", err)
	}
}

func mustParse(t *testing.T, query string) *ast.QueryDocument {
	t.Helper()
	doc, err := parseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func depthOf(t *testing.T, query string) int {
	t.Helper()
	doc, err := parseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	return selectionDepth(doc.Operations[0].SelectionSet, doc.Fragments, 0)
}

func TestPresentError(t *testing.T) {
	t.Parallel()
	ctx := requestid.With(context.Background(), "req-9")
	tests := []struct {
		name    string
		err     error
		message string
		ext     map[string]any
	}{
		{
			name:    "an outage never carries driver text",
			err:     fmt.Errorf("postgres: %w: secret driver text", ledger.ErrUnavailable),
			message: "Dependency unavailable",
			ext: map[string]any{
				"code": "UNAVAILABLE", "reason": "dependency-unavailable", "type": problem.TypeOf("dependency-unavailable"),
				"title": "Dependency unavailable", "status": 503, "requestId": "req-9",
			},
		},
		{
			name: "a declined chain names its posting and results, a pending write its retry delay",
			err: &ledger.Problem{
				Kind: ledger.ErrRejected, Reason: "exceeds-credits", Detail: "Posting 2 would take a below zero.",
				Posting: 1, Results: []string{"linked_event_failed", "exceeds_credits"}, RetryAfter: 1500 * time.Millisecond,
			},
			message: "Posting 2 would take a below zero.",
			ext: map[string]any{
				"code": "REJECTED", "reason": "exceeds-credits", "type": problem.TypeOf("exceeds-credits"),
				"title": "Declined: exceeds credits", "status": 422, "detail": "Posting 2 would take a below zero.",
				"requestId": "req-9", "retryAfter": 2, "posting": 1, "results": []string{"linked_event_failed", "exceeds_credits"},
			},
		},
		{
			name:    "a bad scalar value is invalid",
			err:     gqlerror.WrapPath(nil, fmt.Errorf("%w: Amount must be a JSON string, got float64", ledger.ErrInvalid)),
			message: "Amount must be a JSON string, got float64",
			ext: map[string]any{
				"code": "INVALID", "reason": "invalid-request", "type": problem.TypeOf("invalid-request"), "title": "Invalid request",
				"status": 400, "detail": "Amount must be a JSON string, got float64", "requestId": "req-9",
			},
		},
		{
			name:    "anything else is internal",
			err:     fmt.Errorf("panic: %v", "boom"),
			message: "Internal Server Error",
			ext:     map[string]any{"code": "INTERNAL", "type": "about:blank", "title": "Internal Server Error", "status": 500, "requestId": "req-9"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := presentError(ctx, tt.err)
			if got.Message != tt.message {
				t.Errorf("message = %q, want %q", got.Message, tt.message)
			}
			if diff := cmp.Diff(tt.ext, got.Extensions); diff != "" {
				t.Errorf("extensions (-want +got):\n%s", diff)
			}
			if strings.Contains(fmt.Sprint(got), "secret") {
				t.Errorf("driver text leaked: %v", got)
			}
		})
	}
}
