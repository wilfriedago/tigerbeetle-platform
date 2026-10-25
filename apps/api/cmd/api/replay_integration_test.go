//go:build integration

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// wireProblem is the problem+json body as a client reads it.
type wireProblem struct {
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Status    int      `json:"status"`
	Detail    string   `json:"detail,omitempty"`
	Instance  string   `json:"instance,omitempty"`
	RequestID string   `json:"requestId,omitempty"`
	Posting   *int     `json:"posting,omitempty"`
	Results   []string `json:"results,omitempty"`
}

// rawDo sends a request exactly as given: body bytes and headers, nothing added.
func (h *harness) rawDo(method, path, body string, headers map[string]string) (status int, header http.Header, raw []byte) {
	h.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ = io.ReadAll(res.Body)
	return res.StatusCode, res.Header, raw
}

// TestReplayPinsTheWire replays a fixed list of requests and pins what a client sees: status,
// Content-Type, X-Request-Id, Retry-After, X-Content-Type-Options and the whole problem body. It
// guards the REST contract that ADR-0018 keeps unchanged across the move to Huma.
func TestReplayPinsTheWire(t *testing.T) {
	h := newHarness(t)
	const base = "/v1/ledgers/replay"
	posting := func(n int) *int { return &n }
	missingKey := "every write that moves money requires an Idempotency-Key header; reuse it to retry safely"
	fund := `{"postings":[{"source":"world","destination":"users:1:wallet","asset":"EUR/2","amount":"100"}]}`
	declined := `{"postings":[{"source":"world","destination":"users:1:wallet","asset":"EUR/2","amount":"5"},` +
		`{"source":"users:1:wallet","destination":"merchants:1:main","asset":"EUR/2","amount":"999999"}]}`
	lost := `{"postings":[{"source":"world","destination":"users:2:wallet","asset":"EUR/2","amount":"7"}]}`

	steps := []struct {
		name           string
		before         func()
		method, path   string
		body           string
		headers        map[string]string
		wantStatus     int
		wantProblem    *wireProblem
		wantRetryAfter bool
		wantAllow      string
	}{
		{name: "create the ledger", method: http.MethodPost, path: base, body: `{}`, wantStatus: 201},
		{
			name: "empty body", method: http.MethodPost, path: base, wantStatus: 400,
			wantProblem: &wireProblem{Type: "/problems/invalid-request", Title: "Invalid request", Status: 400, Detail: "request body is required"},
		},
		{
			name: "name taken", method: http.MethodPost, path: base, body: `{}`, wantStatus: 409,
			wantProblem: &wireProblem{Type: "/problems/already-exists", Title: "Already exists", Status: 409},
		},
		{
			name: "unknown ledger", method: http.MethodGet, path: "/v1/ledgers/nope", wantStatus: 404,
			wantProblem: &wireProblem{Type: "/problems/not-found", Title: "Not found", Status: 404},
		},
		{
			name: "undecodable limit", method: http.MethodGet, path: base + "/transactions?limit=abc", wantStatus: 400,
			wantProblem: &wireProblem{Type: "/problems/invalid-request", Title: "Invalid request", Status: 400, Detail: "query.limit: invalid integer"},
		},
		{
			name: "bad cursor", method: http.MethodGet, path: base + "/transactions?cursor=not-a-cursor", wantStatus: 400,
			wantProblem: &wireProblem{Type: "/problems/invalid-request", Title: "Invalid request", Status: 400, Detail: "cursor is not one returned by this API"},
		},
		{
			name: "write without Idempotency-Key", method: http.MethodPost, path: base + "/transactions", body: fund, wantStatus: 400,
			wantProblem: &wireProblem{Type: "/problems/missing-idempotency-key", Title: "Idempotency-Key required", Status: 400, Detail: missingKey},
		},
		{
			name: "fund, with a form Content-Type as curl -d sends", method: http.MethodPost, path: base + "/transactions", body: fund,
			headers:    map[string]string{"Idempotency-Key": "fund-1", "Content-Type": "application/x-www-form-urlencoded"},
			wantStatus: 201,
		},
		{
			name: "declined chain", method: http.MethodPost, path: base + "/transactions", body: declined,
			headers: map[string]string{"Idempotency-Key": "declined-1"}, wantStatus: 422,
			wantProblem: &wireProblem{
				Type: "/problems/exceeds-credits", Title: "Declined: exceeds credits", Status: 422, Posting: posting(1),
				Results: []string{"linked_event_failed", "exceeds_credits"},
			},
		},
		{
			name: "write that never reached the cluster", before: func() { h.faults.dropBefore.Store(true) },
			method: http.MethodPost, path: base + "/transactions", body: lost,
			headers: map[string]string{"Idempotency-Key": "lost-1"}, wantStatus: 503, wantRetryAfter: true,
			wantProblem: &wireProblem{
				Type: "/problems/write-pending", Title: "Write pending", Status: 503,
				Detail: "TigerBeetle or Postgres did not answer in time. The write is recorded and will be completed or settled " +
					"by the reconciler; retry with the same Idempotency-Key to get its outcome",
			},
		},
		{
			name: "retry while pending", method: http.MethodPost, path: base + "/transactions", body: lost,
			headers: map[string]string{"Idempotency-Key": "lost-1"}, wantStatus: 409, wantRetryAfter: true,
			wantProblem: &wireProblem{
				Type: "/problems/request-in-flight", Title: "Request in flight", Status: 409,
				Detail: "a write with this Idempotency-Key is still in progress; retry after the delay to get its outcome",
			},
		},
		{
			name: "unknown route", method: http.MethodGet, path: "/v1/nope", wantStatus: 404,
			wantProblem: &wireProblem{Type: "about:blank", Title: "Not found", Status: 404},
		},
		{
			name: "wrong method", method: http.MethodDelete, path: "/healthz", wantStatus: 405, wantAllow: "GET",
			wantProblem: &wireProblem{Type: "about:blank", Title: "Method not allowed", Status: 405},
		},
	}
	for i, s := range steps {
		if s.before != nil {
			s.before()
		}
		requestID := "replay-" + strconv.Itoa(i)
		headers := map[string]string{"X-Request-Id": requestID, "Content-Type": "application/json"}
		for k, v := range s.headers {
			headers[k] = v
		}
		status, header, raw := h.rawDo(s.method, s.path, s.body, headers)
		if status != s.wantStatus {
			t.Fatalf("%s: status = %d, want %d: %s", s.name, status, s.wantStatus, raw)
		}
		if got := header.Get("X-Request-Id"); got != requestID {
			t.Errorf("%s: X-Request-Id = %q, want %q", s.name, got, requestID)
		}
		if got := header.Get("Retry-After"); (got != "") != s.wantRetryAfter {
			t.Errorf("%s: Retry-After = %q, want present %v", s.name, got, s.wantRetryAfter)
		}
		if s.wantAllow != "" && !strings.Contains(header.Get("Allow"), s.wantAllow) {
			t.Errorf("%s: Allow = %q, want it to contain %q", s.name, header.Get("Allow"), s.wantAllow)
		}
		if s.wantProblem == nil {
			if ct := header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("%s: Content-Type = %q, want application/json", s.name, ct)
			}
			continue
		}
		if ct := header.Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("%s: Content-Type = %q, want application/problem+json", s.name, ct)
		}
		if header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing X-Content-Type-Options: nosniff", s.name)
		}
		var got wireProblem
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("%s: decode %s: %v", s.name, raw, err)
		}
		want := *s.wantProblem
		want.RequestID = requestID
		want.Instance, _, _ = strings.Cut(s.path, "?")
		if want.Detail == "" && got.Detail != "" && want.Type == "/problems/exceeds-credits" {
			// The decline names the account and asset in prose; the members above pin the rest.
			want.Detail = got.Detail
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s: problem mismatch (-want +got):\n%s", s.name, diff)
		}
	}
}
