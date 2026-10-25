package problem

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

func TestFromSentinels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want Problem
	}{
		{
			name: "not found",
			err:  fmt.Errorf("get ledger: %w", ledger.ErrNotFound),
			want: Problem{Kind: KindNotFound, Reason: "not-found", Title: "Not found", Status: 404, Posting: -1},
		},
		{
			name: "already exists, driver text joined",
			err:  errors.Join(ledger.ErrAlreadyExists, errors.New("duplicate key value violates unique constraint")),
			want: Problem{Kind: KindAlreadyExists, Reason: "already-exists", Title: "Already exists", Status: 409, Posting: -1},
		},
		{
			name: "conflict",
			err:  ledger.ErrConflict,
			want: Problem{Kind: KindConflict, Reason: "conflict", Title: "Conflict", Status: 409, Posting: -1},
		},
		{
			name: "rejected",
			err:  ledger.ErrRejected,
			want: Problem{Kind: KindRejected, Reason: "rejected", Title: "Rejected", Status: 422, Posting: -1},
		},
		{
			name: "unavailable, driver text wrapped",
			err:  fmt.Errorf("postgres: %w: dial tcp 10.0.0.1:5432: connection refused", ledger.ErrUnavailable),
			want: Problem{Kind: KindUnavailable, Reason: "dependency-unavailable", Title: "Dependency unavailable", Status: 503, Posting: -1},
		},
		{
			name: "anything else",
			err:  errors.New("something else"),
			want: Problem{Kind: KindInternal, Title: "Internal server error", Status: 500, Posting: -1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, From(tt.err)); diff != "" {
				t.Fatalf("From mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFromKeepsOnlyTheDomainDetail(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("create ledger %q: %w", "Bad", fmt.Errorf("%w: ledger name %q must match the pattern", ledger.ErrInvalid, "Bad"))
	p := From(err)
	if p.Status != 400 || p.Type() != "/problems/invalid-request" {
		t.Fatalf("status %d type %q, want 400 /problems/invalid-request", p.Status, p.Type())
	}
	if want := `ledger name "Bad" must match the pattern`; p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func TestFromDomainProblem(t *testing.T) {
	t.Parallel()
	declined := &ledger.Problem{
		Kind:    ledger.ErrRejected,
		Reason:  "exceeds-credits",
		Detail:  "Posting 2 would take users:1:wallet below zero in EUR/2.",
		Posting: 1,
		Results: []string{"linked_event_failed", "exceeds_credits"},
	}
	want := Problem{
		Kind: KindRejected, Reason: "exceeds-credits", Title: "Declined: exceeds credits", Status: 422,
		Detail: declined.Detail, Posting: 1, Results: declined.Results,
	}
	if diff := cmp.Diff(want, From(fmt.Errorf("create transaction: %w", declined))); diff != "" {
		t.Fatalf("declined mismatch (-want +got):\n%s", diff)
	}

	inFlight := &ledger.Problem{Kind: ledger.ErrConflict, Reason: "request-in-flight", Posting: -1, RetryAfter: 1500 * time.Millisecond}
	got := From(inFlight)
	if got.Status != 409 || got.RetryAfter != 1500*time.Millisecond || got.Posting != -1 || got.Type() != "/problems/request-in-flight" {
		t.Fatalf("in flight: %+v", got)
	}
}

func TestTitleFor(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"exceeds-credits":     "Declined: exceeds credits",
		"not-found":           "Not found",
		"some-future-reason":  "Some future reason",
		"":                    "Problem",
		"history-not-enabled": "History not enabled",
	}
	for reason, want := range tests {
		if got := TitleFor(reason); got != want {
			t.Errorf("TitleFor(%q) = %q, want %q", reason, got, want)
		}
	}
}

func TestBlank(t *testing.T) {
	t.Parallel()
	p := Blank(405, "Method not allowed")
	if p.Type() != TypeBlank || p.Kind != KindInvalid || p.Status != 405 || p.Posting != -1 {
		t.Fatalf("Blank(405) = %+v", p)
	}
	if TypeOf("") != "" || TypeOf("exceeds-credits") != "/problems/exceeds-credits" {
		t.Fatalf("TypeOf: %q %q", TypeOf(""), TypeOf("exceeds-credits"))
	}
}
