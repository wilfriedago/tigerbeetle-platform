package restapi

import (
	"bytes"
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

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/authn/authntest"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// thing and its operations exercise the builder without any use case.
type thing struct {
	Name   scalar.LedgerName `json:"name"`
	Limit  int               `json:"limit"`
	Amount scalar.Amount     `json:"amount" doc:"How much."`
	Tags   []string          `json:"tags"`
	Note   *string           `json:"note,omitempty"`
	Meta   scalar.Metadata   `json:"metadata,omitempty"`
}

type thingRequest struct {
	Code  int      `json:"code" minimum:"1" maximum:"9"`
	Items []string `json:"items,omitempty"`
}

func newTestHandler(t *testing.T) (http.Handler, huma.API) {
	t.Helper()
	mux := http.NewServeMux()
	api := NewAPI(mux)
	Register(api, huma.Operation{
		OperationID: "getThing", Method: http.MethodGet, Path: "/things/{name}", Tags: []string{"ledgers"},
		Responses: Success(http.StatusOK, "The thing."),
	}, func(_ context.Context, in *struct {
		Name  scalar.LedgerName `path:"name" doc:"The ledger name."`
		Limit int               `query:"limit" minimum:"1" maximum:"100" default:"25" doc:"Page size."`
	},
	) (*struct{ Body thing }, error) {
		return &struct{ Body thing }{Body: thing{Name: in.Name, Limit: in.Limit, Amount: "10", Tags: []string{}}}, nil
	})
	Register(api, huma.Operation{
		OperationID: "createThing", Method: http.MethodPost, Path: "/things", Tags: []string{"ledgers"},
		DefaultStatus: http.StatusCreated, Responses: Success(http.StatusCreated, "Created."),
	}, func(_ context.Context, in *struct {
		Key  string `header:"Idempotency-Key" required:"true" minLength:"1" maxLength:"255"`
		Body thingRequest
	},
	) (*struct{ Body thing }, error) {
		switch in.Body.Code {
		case 1:
			return nil, fmt.Errorf("create: %w", &ledger.Problem{
				Kind: ledger.ErrRejected, Reason: "exceeds-credits", Detail: "posting 1 is short", Posting: 0,
				Results: []string{"exceeds_credits", "linked_event_failed"},
			})
		case 2:
			return nil, &ledger.Problem{Kind: ledger.ErrConflict, Reason: "request-in-flight", Posting: -1, RetryAfter: 1500 * time.Millisecond}
		case 3:
			return nil, fmt.Errorf("insert: %w: pq: secret driver text", ledger.ErrUnavailable)
		case 4:
			return nil, errors.New("pq: secret driver text")
		case 5:
			panic("boom")
		}
		return &struct{ Body thing }{Body: thing{Name: "made", Limit: in.Body.Code, Amount: "1", Tags: in.Body.Items}}, nil
	})
	return Handler(mux, slog.New(slog.NewTextHandler(io.Discard, nil)), authntest.Default()), api
}

func do(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader = http.NoBody
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func problemOf(t *testing.T, rec *httptest.ResponseRecorder) problemBody {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != problemMediaType {
		t.Fatalf("Content-Type = %q, want %q; body %s", ct, problemMediaType, rec.Body.String())
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing X-Content-Type-Options: nosniff")
	}
	var p problemBody
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem %q: %v", rec.Body.String(), err)
	}
	return p
}

func intPtr(i int) *int { return &i }

func TestHandlerErrorsBecomeProblems(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(t)
	tests := []struct {
		name           string
		body           string
		want           problemBody
		wantRetryAfter string
	}{
		{
			name: "declined chain keeps posting and results",
			body: `{"code":1}`,
			want: problemBody{
				Type: problem.TypeOf("exceeds-credits"), Title: "Declined: exceeds credits", Status: 422, Detail: "posting 1 is short",
				Instance: "/things", RequestID: "req-1", Posting: intPtr(0), Results: []string{"exceeds_credits", "linked_event_failed"},
			},
		},
		{
			name:           "in flight carries Retry-After rounded up",
			body:           `{"code":2}`,
			want:           problemBody{Type: problem.TypeOf("request-in-flight"), Title: "Request in flight", Status: 409, Instance: "/things", RequestID: "req-1"},
			wantRetryAfter: "2",
		},
		{
			name: "dependency down hides the driver text",
			body: `{"code":3}`,
			want: problemBody{Type: problem.TypeOf("dependency-unavailable"), Title: "Dependency unavailable", Status: 503, Instance: "/things", RequestID: "req-1"},
		},
		{
			name: "unknown error is an internal problem without detail",
			body: `{"code":4}`,
			want: problemBody{Type: "about:blank", Title: "Internal Server Error", Status: 500, Instance: "/things", RequestID: "req-1"},
		},
		{
			name: "panic is an internal problem",
			body: `{"code":5}`,
			want: problemBody{Type: "about:blank", Title: "Internal Server Error", Status: 500, Instance: "/things", RequestID: "req-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, h, http.MethodPost, "/things", tt.body, map[string]string{"Idempotency-Key": "k", "X-Request-Id": "req-1"})
			if rec.Code != tt.want.Status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.want.Status, rec.Body.String())
			}
			if diff := cmp.Diff(tt.want, problemOf(t, rec)); diff != "" {
				t.Fatalf("problem mismatch (-want +got):\n%s", diff)
			}
			if got := rec.Header().Get("Retry-After"); got != tt.wantRetryAfter {
				t.Errorf("Retry-After = %q, want %q", got, tt.wantRetryAfter)
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Errorf("driver text leaked: %s", rec.Body.String())
			}
		})
	}
}

func TestInputErrorsAreInvalidRequests(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(t)
	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		wantDetail   string
		wantErrors   []InputError
		wantInstance string
	}{
		{
			name: "unparsable integer", method: http.MethodGet, path: "/things/eu?limit=abc",
			wantDetail:   "query.limit: invalid integer",
			wantErrors:   []InputError{{Detail: "invalid integer", Parameter: "limit", In: "query"}},
			wantInstance: "/things/eu",
		},
		{
			name: "instance keeps the escaped path", method: http.MethodGet, path: "/things/a%20b?limit=abc",
			wantDetail:   "query.limit: invalid integer",
			wantErrors:   []InputError{{Detail: "invalid integer", Parameter: "limit", In: "query"}},
			wantInstance: "/things/a%20b",
		},
		{
			name: "missing required body", method: http.MethodPost, path: "/things",
			wantDetail: "request body is required", wantInstance: "/things",
		},
		{
			name: "malformed JSON points at the whole body", method: http.MethodPost, path: "/things", body: `{"code":`,
			wantDetail:   "body: malformed JSON at byte 8: unexpected end of JSON input",
			wantErrors:   []InputError{{Detail: "malformed JSON at byte 8: unexpected end of JSON input", Pointer: "#"}},
			wantInstance: "/things",
		},
		{
			name: "wrong type points at the member and names no Go type", method: http.MethodPost, path: "/things",
			body:         `{"code":7,"items":["a",1]}`,
			wantDetail:   "body/items/1: expected string, got number",
			wantErrors:   []InputError{{Detail: "expected string, got number", Pointer: "#/items/1"}},
			wantInstance: "/things",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, h, tt.method, tt.path, tt.body, map[string]string{"Idempotency-Key": "k", "X-Request-Id": "req-1"})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			want := problemBody{
				Type: problem.TypeOf("invalid-request"), Title: "Invalid request", Status: 400, Detail: tt.wantDetail,
				Instance: tt.wantInstance, RequestID: "req-1", Errors: tt.wantErrors,
			}
			if diff := cmp.Diff(want, problemOf(t, rec)); diff != "" {
				t.Fatalf("problem mismatch (-want +got):\n%s", diff)
			}
			if strings.Contains(rec.Body.String(), "Go struct") || strings.Contains(rec.Body.String(), "thingRequest") {
				t.Errorf("a Go type leaked: %s", rec.Body.String())
			}
		})
	}
}

// The other errors Huma raises keep their status: a body over the limit is a 413, not a 400.
func TestOtherStatusesHumaRaisesKeepTheirStatus(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(t)
	body := `{"code":7,"items":["` + strings.Repeat("a", 1<<20) + `"]}`
	rec := do(t, h, http.MethodPost, "/things", body, map[string]string{"Idempotency-Key": "k"})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413: %.200s", rec.Code, rec.Body.String())
	}
	p := problemOf(t, rec)
	if p.Type != problem.TypeBlank || p.Title != "Request Entity Too Large" || p.Status != 413 || p.Detail == "" {
		t.Fatalf("problem = %+v", p)
	}
}

func TestExtensionMembers(t *testing.T) {
	t.Parallel()
	e := &Error{Problem: problem.Blank(http.StatusServiceUnavailable), Instance: "/readyz", Extensions: map[string]any{"dependencies": map[string]string{"postgres": "down"}}}
	got, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"type":"about:blank","title":"Service Unavailable","status":503,"instance":"/readyz","dependencies":{"postgres":"down"}}`
	if string(got) != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
	e.Extensions = map[string]any{"status": "not_ready"}
	if _, err := json.Marshal(e); err == nil {
		t.Fatal("an extension member reusing status was written")
	}
}

func TestValidationStaysInTheDomain(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(t)
	// Out of the documented range and with no Idempotency-Key: Huma documents the constraints but
	// does not enforce them; the handler decides.
	rec := do(t, h, http.MethodPost, "/things", `{"code":42,"items":["a"],"unknown":true}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodGet, "/things/eu?limit=500", "", nil)
	var got thing
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Limit != 500 {
		t.Fatalf("limit passed through as %d (%v): %s", got.Limit, err, rec.Body.String())
	}
	rec = do(t, h, http.MethodGet, "/things/eu", "", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Limit != 25 {
		t.Fatalf("default limit = %d (%v), want 25", got.Limit, err)
	}
}

func TestBodiesAreJSONWhateverTheContentType(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(t)
	for _, ct := range []string{"", "application/x-www-form-urlencoded", "text/plain", "application/json; charset=utf-8"} {
		rec := do(t, h, http.MethodPost, "/things", `{"code":7}`, map[string]string{"Content-Type": ct})
		if rec.Code != http.StatusCreated {
			t.Errorf("Content-Type %q: status = %d, want 201: %s", ct, rec.Code, rec.Body.String())
		}
	}
}

func TestUnknownRoutes(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(t)
	tests := []struct {
		name, method, path string
		wantStatus         int
		wantTitle          string
		wantAllow          string
	}{
		{name: "unknown path", method: http.MethodGet, path: "/nope", wantStatus: 404, wantTitle: "Not Found"},
		{name: "wrong method", method: http.MethodDelete, path: "/things", wantStatus: 405, wantTitle: "Method Not Allowed", wantAllow: "POST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := do(t, h, tt.method, tt.path, "", map[string]string{"X-Request-Id": "req-123"})
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantAllow != "" && !strings.Contains(rec.Header().Get("Allow"), tt.wantAllow) {
				t.Errorf("Allow = %q, want it to contain %q", rec.Header().Get("Allow"), tt.wantAllow)
			}
			want := problemBody{Type: "about:blank", Title: tt.wantTitle, Status: tt.wantStatus, RequestID: "req-123", Instance: tt.path}
			if diff := cmp.Diff(want, problemOf(t, rec)); diff != "" {
				t.Fatalf("problem mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler(t)
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
			rec := do(t, h, http.MethodGet, "/things/eu", "", map[string]string{"X-Request-Id": tt.supplied})
			got := rec.Header().Get("X-Request-Id")
			if got == "" || (got == tt.supplied) != tt.wantKept {
				t.Fatalf("X-Request-Id = %q (supplied %q), wantKept %v", got, tt.supplied, tt.wantKept)
			}
		})
	}
}

func TestSpec(t *testing.T) {
	t.Parallel()
	_, api := newTestHandler(t)
	first, err := Spec(api)
	if err != nil {
		t.Fatal(err)
	}
	_, again := newTestHandler(t)
	second, err := Spec(again)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two renders of the same API differ")
	}
	spec := string(first)
	for _, want := range []string{
		specHeader,
		"openapi: 3.1.0",
		"security: []",
		"summary: A ledger API that makes TigerBeetle easier to work with.",
		"$ref: '#/components/responses/Problem'",
		"application/problem+json:",
		"format: uri-reference",
		"propertyNames",
		"X-Request-Id:",
		"Retry-After:",
		"name: X-Request-Id",
	} {
		if !strings.Contains(spec, want) && !strings.Contains(spec, strings.ReplaceAll(want, "'", `"`)) {
			t.Errorf("spec lacks %q", want)
		}
	}
	for _, unwanted := range []string{"additionalProperties: false", "format: int64", "$schema", "nullable"} {
		if strings.Contains(spec, unwanted) {
			t.Errorf("spec contains %q", unwanted)
		}
	}
	if strings.Index(spec, "name: operations") > strings.Index(spec, "name: settings") {
		t.Error("tags are not in their published order")
	}
}
