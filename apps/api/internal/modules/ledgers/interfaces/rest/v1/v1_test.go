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
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn/authntest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// fakeLedgers answers the ledger use cases from fields; a nil error with a zero view is a success.
type fakeLedgers struct {
	application.LedgerCommands
	application.SettingsCommands
	view application.LedgerView
	err  error
}

func (f fakeLedgers) Create(_ context.Context, name string, metadata map[string]string) (application.LedgerView, error) {
	v := f.view
	v.Name, v.Metadata = name, metadata
	return v, f.err
}

func (f fakeLedgers) Get(context.Context, string) (application.LedgerView, error) {
	return f.view, f.err
}

func (f fakeLedgers) List(context.Context, string, int) ([]application.LedgerView, string, error) {
	return []application.LedgerView{f.view}, "", f.err
}

func serve(t *testing.T, f fakeLedgers, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	api := restapi.NewAPI(mux)
	Register(api, Deps{LedgerCommands: f, LedgerQueries: f, SettingsCommands: f})
	h := restapi.Handler(mux, slog.New(slog.NewTextHandler(io.Discard, nil)), restapi.Options{Auth: authntest.Default()})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	return rec
}

func TestLedgerOperations(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	f := fakeLedgers{view: application.LedgerView{
		Ledger:    ledger.Ledger{Name: "eu", Metadata: map[string]string{}, CreatedAt: created},
		Footprint: ledger.Footprint{Assets: []string{"EUR/2"}, Accounts: 2},
	}}

	rec := serve(t, f, http.MethodPost, "/v1/ledgers/eu", `{"metadata":{"env":"test"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "eu" || got["accounts"] != float64(2) || fmt.Sprint(got["assets"]) != "[EUR/2]" || fmt.Sprint(got["metadata"]) != "map[env:test]" {
		t.Fatalf("create body = %v", got)
	}

	rec = serve(t, f, http.MethodGet, "/v1/ledgers", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"data":[{"name":"eu"`) || strings.Contains(rec.Body.String(), `"next"`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
}

func TestErrorsNeverCarryDriverText(t *testing.T) {
	t.Parallel()
	f := fakeLedgers{err: fmt.Errorf("postgres: %w: secret driver text", ledger.ErrUnavailable)}
	rec := serve(t, f, http.MethodGet, "/v1/ledgers/eu", "")
	if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestPutRuleRefusesCodeZeroItself(t *testing.T) {
	t.Parallel()
	// SettingsCommands is nil in the fake: reaching the use case would panic and answer 500.
	rec := serve(t, fakeLedgers{}, http.MethodPut, "/v1/ledgers/eu/rules/users:", `{"limit":"none","code":0}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "TigerBeetle rejects code 0") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
