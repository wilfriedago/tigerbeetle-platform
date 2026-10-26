package v1

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

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn/authntest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// newTestServer serves the bookkeeping operations on their own mux, with the production middleware.
func newTestServer(t *testing.T, deps Deps) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	api := restapi.NewAPI(mux)
	Register(api, deps)
	return restapi.Handler(mux, slog.New(slog.NewTextHandler(io.Discard, nil)), authntest.Default())
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return v
}

// failingQueries answers every transaction read with an error that wraps driver text.
type failingQueries struct {
	application.TransactionQueries
}

func (failingQueries) Get(context.Context, string, string) (application.TransactionView, error) {
	return application.TransactionView{}, fmt.Errorf("tigerbeetle: lookup_transfers: %w: secret driver text", ledger.ErrUnavailable)
}

func TestErrorsNeverCarryDriverText(t *testing.T) {
	t.Parallel()
	h := newTestServer(t, Deps{TransactionQueries: failingQueries{}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/ledgers/eu/transactions/0192f3a1b7c94e2a8d5f6c0b1e2d7c2e", http.NoBody))
	if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
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
	req.Header.Set("Content-Type", "application/json")
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
	h := newTestServer(t, Deps{})
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
				Type: problem.TypeOf("missing-idempotency-key"), Status: 400,
				Detail: "every write that moves money requires an Idempotency-Key header; reuse it to retry safely",
			},
		},
		{
			name: "revert without Idempotency-Key and without body",
			path: "/v1/ledgers/eu/transactions/0192f3a1b7c94e2a8d5f6c0b1e2d7c2e/revert",
			want: problemBody{
				Type: problem.TypeOf("missing-idempotency-key"), Status: 400,
				Detail: "every write that moves money requires an Idempotency-Key header; reuse it to retry safely",
			},
		},
		{
			name: "void without Idempotency-Key",
			path: "/v1/ledgers/eu/holds/0192f3a1b7c94e2a8d5f6c0b1e2d7c2e/void",
			want: problemBody{
				Type: problem.TypeOf("missing-idempotency-key"), Status: 400,
				Detail: "every write that moves money requires an Idempotency-Key header; reuse it to retry safely",
			},
		},
		{
			name: "hold with a timeout out of range",
			path: "/v1/ledgers/eu/holds", key: "k2",
			body: `{"source":"a","destination":"b","asset":"EUR/2","amount":"1","timeoutSeconds":4294967296}`,
			want: problemBody{Type: problem.TypeOf("invalid-request"), Status: 400, Detail: "timeoutSeconds must be between 0 and 4294967295"},
		},
		{
			name: "posting that does not parse names its index",
			path: "/v1/ledgers/eu/transactions", key: "k1",
			body: `{"postings":[{"source":"world","destination":"a","asset":"EUR/2","amount":"1"},` +
				`{"source":"world","destination":"a b","asset":"EUR/2","amount":"1"}]}`,
			want:    problemBody{Type: problem.TypeOf("invalid-request"), Status: 400, Posting: &posting},
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
