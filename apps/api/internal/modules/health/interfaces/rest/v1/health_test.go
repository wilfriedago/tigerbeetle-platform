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

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/readiness"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

type readyFunc func(ctx context.Context) error

func up(context.Context) error { return nil }

// newTestServer serves the health operations on their own mux, with the production middleware.
func newTestServer(t *testing.T, deps Deps) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	api := restapi.NewAPI(mux)
	Register(api, deps)
	return restapi.Handler(mux, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func statusDeps(pg, tb readyFunc) Deps {
	checker := readiness.New(50 * time.Millisecond)
	checker.Require(ProbePostgres, readiness.Probe(pg))
	checker.Require(ProbeTigerBeetle, readiness.Probe(tb))
	return Deps{Readiness: checker, Service: Service{Name: "test-service", Version: "1.2.3"}}
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
	h := newTestServer(t, statusDeps(down, down))
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
			h := newTestServer(t, statusDeps(tt.pg, tt.tb))

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
