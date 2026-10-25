package v1

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

type readyFunc func(ctx context.Context) error

func (f readyFunc) Ready(ctx context.Context) error { return f(ctx) }

// metadataFake only needs Ready for the status use case.
type metadataFake struct {
	readyFunc
}

func (metadataFake) CreateLedger(context.Context, ledger.Ledger) (ledger.Ledger, error) {
	return ledger.Ledger{}, errors.New("not implemented")
}

func (metadataFake) GetLedger(context.Context, string) (ledger.Ledger, error) {
	return ledger.Ledger{}, errors.New("not implemented")
}

func up(context.Context) error { return nil }

// newTestServer serves the version 1 operations on their own mux, with the production middleware.
func newTestServer(t *testing.T, deps Deps) (http.Handler, huma.API) {
	t.Helper()
	mux := http.NewServeMux()
	api := restapi.NewAPI(mux)
	Register(api, deps)
	return restapi.Handler(mux, slog.New(slog.NewTextHandler(io.Discard, nil))), api
}

func statusDeps(pg, tb readyFunc) Deps {
	return Deps{
		Status:  app.NewStatus(tb, metadataFake{readyFunc: pg}, 50*time.Millisecond),
		Service: Service{Name: "test-service", Version: "1.2.3"},
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestLiveness(t *testing.T) {
	t.Parallel()
	down := readyFunc(func(context.Context) error { return errors.New("down") })
	h, _ := newTestServer(t, statusDeps(down, down))
	rec := get(t, h, "/healthz")
	if rec.Code != http.StatusOK || decode[Liveness](t, rec).Status != "ok" {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("missing X-Request-Id header")
	}
}

func TestReadinessAndInfo(t *testing.T) {
	t.Parallel()
	errDown := errors.New("connection refused")
	slow := readyFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	tests := []struct {
		name          string
		pg, tb        readyFunc
		wantReadyCode int
		wantStatus    string
		wantTB        scalar.DependencyStatus
		wantTBError   string
	}{
		{name: "all up", pg: up, tb: up, wantReadyCode: 200, wantStatus: "ready", wantTB: scalar.DependencyUp},
		{
			name: "tigerbeetle down", pg: up, tb: func(context.Context) error { return errDown },
			wantReadyCode: 503, wantStatus: "not_ready", wantTB: scalar.DependencyDown, wantTBError: "unreachable",
		},
		{
			name: "tigerbeetle timeout", pg: up, tb: slow,
			wantReadyCode: 503, wantStatus: "not_ready", wantTB: scalar.DependencyDown, wantTBError: "timeout",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, _ := newTestServer(t, statusDeps(tt.pg, tt.tb))

			rec := get(t, h, "/readyz")
			if rec.Code != tt.wantReadyCode {
				t.Fatalf("/readyz status = %d, want %d: %s", rec.Code, tt.wantReadyCode, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("/readyz Content-Type = %q", ct)
			}
			ready := decode[Readiness](t, rec)
			if ready.Status != tt.wantStatus {
				t.Errorf("/readyz status field = %q, want %q", ready.Status, tt.wantStatus)
			}
			checkDeps(t, ready.Dependencies, tt.wantTB, tt.wantTBError)

			rec = get(t, h, "/v1/_info")
			if rec.Code != http.StatusOK {
				t.Fatalf("/v1/_info status = %d, want 200", rec.Code)
			}
			info := decode[Info](t, rec)
			if info.Service != "test-service" || info.Version != "1.2.3" || info.Runtime == nil || info.Runtime.GoVersion == "" {
				t.Errorf("info = %+v", info)
			}
			checkDeps(t, info.Dependencies, tt.wantTB, tt.wantTBError)
		})
	}
}

func checkDeps(t *testing.T, deps Dependencies, wantTB scalar.DependencyStatus, wantTBError string) {
	t.Helper()
	if deps.Postgres.Status != scalar.DependencyUp {
		t.Errorf("postgres status = %q, want up", deps.Postgres.Status)
	}
	if deps.Tigerbeetle.Status != wantTB {
		t.Errorf("tigerbeetle status = %q, want %q", deps.Tigerbeetle.Status, wantTB)
	}
	var gotErr string
	if deps.Tigerbeetle.Error != nil {
		gotErr = *deps.Tigerbeetle.Error
	}
	if gotErr != wantTBError {
		t.Errorf("tigerbeetle error = %q, want %q", gotErr, wantTBError)
	}
	if strings.Contains(gotErr, "refused") {
		t.Errorf("driver error leaked to the client: %q", gotErr)
	}
}

// registered lists the operations of the committed spec, by tag, as the generated spec must
// declare them.
var registered = []struct {
	id, method, path, tag string
}{
	{"getLiveness", http.MethodGet, "/healthz", "operations"},
	{"getReadiness", http.MethodGet, "/readyz", "operations"},
	{"getInfo", http.MethodGet, "/v1/_info", "operations"},
	{"getReconcilerStatus", http.MethodGet, "/v1/_reconciler", "operations"},
	{"listLedgers", http.MethodGet, "/v1/ledgers", "ledgers"},
	{"getLedger", http.MethodGet, "/v1/ledgers/{ledger}", "ledgers"},
	{"createLedger", http.MethodPost, "/v1/ledgers/{ledger}", "ledgers"},
	{"replaceLedgerMetadata", http.MethodPut, "/v1/ledgers/{ledger}/metadata", "ledgers"},
	{"getLedgerStats", http.MethodGet, "/v1/ledgers/{ledger}/stats", "ledgers"},
	{"listAssets", http.MethodGet, "/v1/ledgers/{ledger}/assets", "ledgers"},
	{"search", http.MethodGet, "/v1/ledgers/{ledger}/search", "ledgers"},
	{"listAccounts", http.MethodGet, "/v1/ledgers/{ledger}/accounts", "accounts"},
	{"getAccount", http.MethodGet, "/v1/ledgers/{ledger}/accounts/{address}", "accounts"},
	{"replaceAccountMetadata", http.MethodPut, "/v1/ledgers/{ledger}/accounts/{address}/metadata", "accounts"},
	{"getAccountHistory", http.MethodGet, "/v1/ledgers/{ledger}/accounts/{address}/history", "accounts"},
	{"aggregateBalances", http.MethodGet, "/v1/ledgers/{ledger}/aggregate/balances", "accounts"},
	{"listSegments", http.MethodGet, "/v1/ledgers/{ledger}/aggregate/segments", "accounts"},
	{"listTransactions", http.MethodGet, "/v1/ledgers/{ledger}/transactions", "transactions"},
	{"createTransaction", http.MethodPost, "/v1/ledgers/{ledger}/transactions", "transactions"},
	{"getTransaction", http.MethodGet, "/v1/ledgers/{ledger}/transactions/{id}", "transactions"},
	{"replaceTransactionMetadata", http.MethodPut, "/v1/ledgers/{ledger}/transactions/{id}/metadata", "transactions"},
	{"revertTransaction", http.MethodPost, "/v1/ledgers/{ledger}/transactions/{id}/revert", "transactions"},
	{"listWriteRequests", http.MethodGet, "/v1/ledgers/{ledger}/requests", "transactions"},
	{"getWriteRequest", http.MethodGet, "/v1/ledgers/{ledger}/requests/{key}", "transactions"},
	{"listHolds", http.MethodGet, "/v1/ledgers/{ledger}/holds", "holds"},
	{"createHold", http.MethodPost, "/v1/ledgers/{ledger}/holds", "holds"},
	{"getHold", http.MethodGet, "/v1/ledgers/{ledger}/holds/{id}", "holds"},
	{"captureHold", http.MethodPost, "/v1/ledgers/{ledger}/holds/{id}/capture", "holds"},
	{"voidHold", http.MethodPost, "/v1/ledgers/{ledger}/holds/{id}/void", "holds"},
	{"listRules", http.MethodGet, "/v1/ledgers/{ledger}/rules", "settings"},
	{"matchRule", http.MethodGet, "/v1/ledgers/{ledger}/rules/match", "settings"},
	{"putRule", http.MethodPut, "/v1/ledgers/{ledger}/rules/{pattern}", "settings"},
	{"deleteRule", http.MethodDelete, "/v1/ledgers/{ledger}/rules/{pattern}", "settings"},
	{"listEventTypes", http.MethodGet, "/v1/ledgers/{ledger}/event-types", "settings"},
	{"createEventType", http.MethodPost, "/v1/ledgers/{ledger}/event-types", "settings"},
}

func TestOperationsAreRegistered(t *testing.T) {
	t.Parallel()
	_, api := newTestServer(t, Deps{})
	found := map[string]bool{}
	for path, item := range api.OpenAPI().Paths {
		for method, op := range map[string]*huma.Operation{
			http.MethodGet: item.Get, http.MethodPost: item.Post, http.MethodPut: item.Put, http.MethodDelete: item.Delete,
		} {
			if op != nil {
				found[method+" "+path+" "+op.OperationID+" "+strings.Join(op.Tags, ",")] = true
			}
		}
	}
	for _, r := range registered {
		key := r.method + " " + r.path + " " + r.id + " " + r.tag
		if !found[key] {
			t.Errorf("operation %s is not registered as %s", r.id, key)
		}
		delete(found, key)
	}
	for extra := range found {
		t.Errorf("unexpected operation %s", extra)
	}
}

// problemBody is the problem+json members the tests compare.
type problemBody struct {
	Type    string `json:"type"`
	Status  int    `json:"status"`
	Detail  string `json:"detail"`
	Posting *int   `json:"posting"`
}

func post(t *testing.T, h http.Handler, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestWritesCheckTheRequestFirst covers the answers a write gives before it reaches a use case;
// the use cases are nil, so reaching one would fail the test with a 500.
func TestWritesCheckTheRequestFirst(t *testing.T) {
	t.Parallel()
	h, _ := newTestServer(t, Deps{})
	posting := 1
	tests := []struct {
		name    string
		path    string
		body    string
		key     string
		want    problemBody
		wantSub string
	}{
		{
			name: "transaction without Idempotency-Key",
			path: "/v1/ledgers/eu/transactions", body: `{"postings":[]}`,
			want: problemBody{
				Type: "/problems/missing-idempotency-key", Status: 400,
				Detail: "every write that moves money requires an Idempotency-Key header; reuse it to retry safely",
			},
		},
		{
			name: "revert without Idempotency-Key and without body",
			path: "/v1/ledgers/eu/transactions/0192f3a1b7c94e2a8d5f6c0b1e2d7c2e/revert",
			want: problemBody{
				Type: "/problems/missing-idempotency-key", Status: 400,
				Detail: "every write that moves money requires an Idempotency-Key header; reuse it to retry safely",
			},
		},
		{
			name: "void without Idempotency-Key",
			path: "/v1/ledgers/eu/holds/0192f3a1b7c94e2a8d5f6c0b1e2d7c2e/void",
			want: problemBody{
				Type: "/problems/missing-idempotency-key", Status: 400,
				Detail: "every write that moves money requires an Idempotency-Key header; reuse it to retry safely",
			},
		},
		{
			name: "hold with a timeout out of range",
			path: "/v1/ledgers/eu/holds", key: "k2",
			body: `{"source":"a","destination":"b","asset":"EUR/2","amount":"1","timeoutSeconds":4294967296}`,
			want: problemBody{Type: "/problems/invalid-request", Status: 400, Detail: "timeoutSeconds must be between 0 and 4294967295"},
		},
		{
			name: "posting that does not parse names its index",
			path: "/v1/ledgers/eu/transactions", key: "k1",
			body: `{"postings":[{"source":"world","destination":"a","asset":"EUR/2","amount":"1"},` +
				`{"source":"world","destination":"a b","asset":"EUR/2","amount":"1"}]}`,
			want:    problemBody{Type: "/problems/invalid-request", Status: 400, Posting: &posting},
			wantSub: "posting 2: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			headers := map[string]string{}
			if tt.key != "" {
				headers["Idempotency-Key"] = tt.key
			}
			rec := post(t, h, tt.path, tt.body, headers)
			got := decode[problemBody](t, rec)
			if rec.Code != tt.want.Status || got.Type != tt.want.Type {
				t.Fatalf("status %d type %q, want %d %q: %s", rec.Code, got.Type, tt.want.Status, tt.want.Type, rec.Body.String())
			}
			if tt.wantSub != "" {
				if !strings.HasPrefix(got.Detail, tt.wantSub) {
					t.Errorf("detail = %q, want prefix %q", got.Detail, tt.wantSub)
				}
				got.Detail, tt.want.Detail = "", ""
			}
			if got.Detail != tt.want.Detail || (got.Posting == nil) != (tt.want.Posting == nil) ||
				(got.Posting != nil && *got.Posting != *tt.want.Posting) {
				t.Errorf("problem = %+v, want %+v", got, tt.want)
			}
		})
	}
}
