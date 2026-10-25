package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

type readyFunc func(ctx context.Context) error

func (f readyFunc) Ready(ctx context.Context) error { return f(ctx) }

// metadataFake only needs Ready for these tests.
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

func newTestHandler(pg, tb readyFunc) http.Handler {
	status := app.NewStatus(tb, metadataFake{readyFunc: pg}, 50*time.Millisecond)
	return NewHandler(Deps{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Status:  status,
		Service: "test-service",
		Version: "1.2.3",
	})
}

func do(t *testing.T, h http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, http.NoBody)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestLiveness(t *testing.T) {
	t.Parallel()
	down := readyFunc(func(context.Context) error { return errors.New("down") })
	rec := do(t, newTestHandler(down, down), http.MethodGet, "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := decode[gen.Liveness](t, rec); got.Status != gen.Ok {
		t.Fatalf("body = %+v", got)
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
		wantStatus    gen.ReadinessStatus
		wantPG        gen.DependencyStatus
		wantTB        gen.DependencyStatus
		wantTBError   string
	}{
		{name: "all up", pg: up, tb: up, wantReadyCode: 200, wantStatus: gen.Ready, wantPG: gen.Up, wantTB: gen.Up},
		{
			name: "tigerbeetle down", pg: up, tb: func(context.Context) error { return errDown },
			wantReadyCode: 503, wantStatus: gen.NotReady, wantPG: gen.Up, wantTB: gen.Down, wantTBError: "unreachable",
		},
		{
			name: "tigerbeetle timeout", pg: up, tb: slow,
			wantReadyCode: 503, wantStatus: gen.NotReady, wantPG: gen.Up, wantTB: gen.Down, wantTBError: "timeout",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newTestHandler(tt.pg, tt.tb)

			rec := do(t, h, http.MethodGet, "/readyz", nil)
			if rec.Code != tt.wantReadyCode {
				t.Fatalf("/readyz status = %d, want %d: %s", rec.Code, tt.wantReadyCode, rec.Body.String())
			}
			ready := decode[gen.Readiness](t, rec)
			if ready.Status != tt.wantStatus {
				t.Errorf("/readyz status field = %q, want %q", ready.Status, tt.wantStatus)
			}
			checkDeps(t, ready.Dependencies, tt.wantPG, tt.wantTB, tt.wantTBError)

			rec = do(t, h, http.MethodGet, "/v1/_info", nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("/v1/_info status = %d, want 200", rec.Code)
			}
			info := decode[gen.Info](t, rec)
			if info.Service != "test-service" || info.Version != "1.2.3" {
				t.Errorf("info = %+v", info)
			}
			checkDeps(t, info.Dependencies, tt.wantPG, tt.wantTB, tt.wantTBError)
		})
	}
}

func checkDeps(t *testing.T, deps gen.Dependencies, wantPG, wantTB gen.DependencyStatus, wantTBError string) {
	t.Helper()
	if deps.Postgres.Status != wantPG {
		t.Errorf("postgres status = %q, want %q", deps.Postgres.Status, wantPG)
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

func TestProblemResponses(t *testing.T) {
	t.Parallel()
	h := newTestHandler(up, up)
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantTitle  string
		wantAllow  string
	}{
		{name: "unknown path", method: http.MethodGet, path: "/nope", wantStatus: 404, wantTitle: "Not found"},
		{name: "wrong method", method: http.MethodPost, path: "/healthz", wantStatus: 405, wantTitle: "Method not allowed", wantAllow: "GET"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, h, tt.method, tt.path, map[string]string{"X-Request-Id": "req-123"})
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Fatalf("content type = %q", ct)
			}
			if tt.wantAllow != "" && !strings.Contains(rec.Header().Get("Allow"), tt.wantAllow) {
				t.Errorf("Allow header = %q, want it to contain %q", rec.Header().Get("Allow"), tt.wantAllow)
			}
			got := decode[gen.Problem](t, rec)
			reqID, instance := "req-123", tt.path
			want := gen.Problem{Type: "about:blank", Title: tt.wantTitle, Status: tt.wantStatus, RequestId: &reqID, Instance: &instance}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("problem mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProblemFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err        error
		wantStatus int
	}{
		{err: ledger.ErrNotFound, wantStatus: 404},
		{err: errors.Join(ledger.ErrAlreadyExists, errors.New("duplicate key value violates unique constraint")), wantStatus: 409},
		{err: ledger.ErrInvalid, wantStatus: 400},
		{err: ledger.ErrUnavailable, wantStatus: 503},
		{err: ledger.ErrConflict, wantStatus: 409},
		{err: ledger.ErrRejected, wantStatus: 422},
		{err: errors.New("something else"), wantStatus: 500},
	}
	for _, tt := range tests {
		p, _ := problemFor(tt.err)
		if p.Status != tt.wantStatus {
			t.Errorf("problemFor(%v).Status = %d, want %d", tt.err, p.Status, tt.wantStatus)
		}
		// Domain-mapped problems carry no detail at all: the wrapped driver text must not leak.
		if p.Detail != nil {
			t.Errorf("problemFor(%v) set Detail %q; driver text must not reach clients", tt.err, *p.Detail)
		}
	}
}

func TestProblemForDomainProblem(t *testing.T) {
	t.Parallel()
	declined := &ledger.Problem{
		Kind:    ledger.ErrRejected,
		Reason:  "exceeds-credits",
		Detail:  "Posting 2 would take users:1:wallet below zero in EUR/2.",
		Posting: 1,
		Results: []string{"linked_event_failed", "exceeds_credits"},
	}
	p, _ := problemFor(fmt.Errorf("create transaction: %w", declined))
	if p.Status != 422 || p.Type != "/problems/exceeds-credits" {
		t.Fatalf("status %d type %q, want 422 /problems/exceeds-credits", p.Status, p.Type)
	}
	if p.Posting == nil || *p.Posting != 1 || p.Results == nil || len(*p.Results) != 2 {
		t.Fatalf("extensions not carried: posting %v results %v", p.Posting, p.Results)
	}
	inFlight := &ledger.Problem{Kind: ledger.ErrConflict, Reason: "request-in-flight", Posting: -1, RetryAfter: 1500 * time.Millisecond}
	p, retry := problemFor(inFlight)
	if p.Status != 409 || retry != 1500*time.Millisecond || p.Posting != nil {
		t.Fatalf("in flight: status %d retry %v posting %v", p.Status, retry, p.Posting)
	}
}

func TestValidationDetailIsClientSafe(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("create ledger %q: %w", "Bad", fmt.Errorf("%w: ledger name %q must match the pattern", ledger.ErrInvalid, "Bad"))
	p, _ := problemFor(err)
	if p.Detail == nil || *p.Detail != `ledger name "Bad" must match the pattern` {
		t.Fatalf("detail = %v", p.Detail)
	}
}

func TestRequestIDValidation(t *testing.T) {
	t.Parallel()
	h := newTestHandler(up, up)
	tests := []struct {
		name     string
		supplied string
		wantKept bool
	}{
		{name: "kept when printable", supplied: "abc-123", wantKept: true},
		{name: "replaced when it has spaces", supplied: "a b", wantKept: false},
		{name: "replaced when too long", supplied: strings.Repeat("x", 129), wantKept: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, h, http.MethodGet, "/healthz", map[string]string{"X-Request-Id": tt.supplied})
			got := rec.Header().Get("X-Request-Id")
			if (got == tt.supplied) != tt.wantKept {
				t.Fatalf("X-Request-Id = %q (supplied %q), wantKept %v", got, tt.supplied, tt.wantKept)
			}
			if got == "" {
				t.Fatal("missing X-Request-Id")
			}
		})
	}
}
