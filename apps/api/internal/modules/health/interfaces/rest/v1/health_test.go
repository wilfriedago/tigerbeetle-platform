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

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
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
	checker.Require(dto.ProbePostgres, readiness.Probe(pg))
	checker.Require(dto.ProbeTigerBeetle, readiness.Probe(tb))
	return Deps{Readiness: checker, Service: dto.Service{Name: "test-service", Version: "1.2.3"}}
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
	if rec.Code != http.StatusOK || decode[dto.Liveness](t, rec).Status != "ok" {
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
			ready := decode[dto.Readiness](t, rec)
			if ready.Status != tt.wantStatus {
				t.Errorf("/readyz status field = %q, want %q", ready.Status, tt.wantStatus)
			}
			checkDeps(t, ready.Dependencies, tt.wantTB, tt.wantTBError)

			rec = get(t, h, "/v1/_info")
			if rec.Code != http.StatusOK {
				t.Fatalf("/v1/_info status = %d, want 200", rec.Code)
			}
			info := decode[dto.Info](t, rec)
			if info.Service != "test-service" || info.Version != "1.2.3" || info.Runtime == nil || info.Runtime.GoVersion == "" {
				t.Errorf("info = %+v", info)
			}
			checkDeps(t, info.Dependencies, tt.wantTB, tt.wantTBError)
		})
	}
}

func checkDeps(t *testing.T, deps dto.Dependencies, wantTB scalar.DependencyStatus, wantTBError string) {
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

// TestReadinessDegraded: a broker that is down makes the service degraded, still 200, because
// readiness gates HTTP traffic and the brokers carry none of it (plan section 13.2). A broker
// that is off does not appear at all.
func TestReadinessDegraded(t *testing.T) {
	t.Parallel()
	deps := statusDeps(up, up)
	deps.Readiness.Optional(dto.ProbeRabbitMQ, func(context.Context) error { return errors.New("secret broker text") })
	deps.Readiness.Optional(dto.ProbeNATS, up)
	rec := get(t, newTestServer(t, deps), "/readyz")
	body := decode[dto.Readiness](t, rec)
	if rec.Code != http.StatusOK || body.Status != "degraded" {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if body.Dependencies.RabbitMQ == nil || body.Dependencies.RabbitMQ.Status != scalar.DependencyDown ||
		body.Dependencies.NATS == nil || body.Dependencies.NATS.Status != scalar.DependencyUp {
		t.Fatalf("dependencies = %+v", body.Dependencies)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("readiness leaks the probe error: %s", rec.Body.String())
	}

	plain := get(t, newTestServer(t, statusDeps(up, up)), "/readyz")
	if strings.Contains(plain.Body.String(), "rabbitmq") || strings.Contains(plain.Body.String(), "nats") {
		t.Fatalf("brokers reported while off: %s", plain.Body.String())
	}
}
