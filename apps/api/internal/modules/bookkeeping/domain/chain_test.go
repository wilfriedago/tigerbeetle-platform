package domain

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func outcomes(results ...string) []TransferOutcome {
	out := make([]TransferOutcome, len(results))
	for i, r := range results {
		out[i] = TransferOutcome{Result: r, Timestamp: uint64(100 + i)}
	}
	return out
}

func TestClassify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		results []string
		verdict Verdict
		failed  int
		result  string
	}{
		{name: "all created", results: []string{ResultCreated, ResultCreated}, verdict: VerdictCommitted, failed: -1},
		{name: "already landed", results: []string{ResultExists, ResultLinkedEventFailed, ResultLinkedEventFailed}, verdict: VerdictCommitted, failed: -1},
		{name: "every transfer exists", results: []string{ResultExists, ResultExists}, verdict: VerdictCommitted, failed: -1},
		{name: "second posting declined", results: []string{ResultLinkedEventFailed, ResultExceedsCredits, ResultLinkedEventFailed}, verdict: VerdictDeclined, failed: 1, result: ResultExceedsCredits},
		{name: "one posting declined", results: []string{ResultExceedsDebits}, verdict: VerdictDeclined, failed: 0, result: ResultExceedsDebits},
		{name: "burned id", results: []string{ResultIDAlreadyFailed, ResultLinkedEventFailed}, verdict: VerdictBurned, failed: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Classify(outcomes(tt.results...))
			if got.Verdict != tt.verdict || got.Failed != tt.failed || got.Result != tt.result {
				t.Fatalf("Classify(%v) = %+v, want verdict %d failed %d result %q", tt.results, got, tt.verdict, tt.failed, tt.result)
			}
			if tt.verdict == VerdictCommitted && tt.results[0] == ResultCreated && got.Timestamp != uint64(100+len(tt.results)-1) {
				t.Fatalf("timestamp = %d, want the latest", got.Timestamp)
			}
		})
	}
	if ResultReason(ResultExceedsCredits) != "exceeds-credits" {
		t.Fatal("ResultReason")
	}
}

func TestStoredProblemRoundTrip(t *testing.T) {
	t.Parallel()
	original := &ledger.Problem{Kind: ledger.ErrRejected, Reason: "exceeds-credits", Detail: "d", Posting: 0, Results: []string{"exceeds_credits"}}
	status, body := ProblemOutcome(original)
	if status != StatusRejected {
		t.Fatalf("status = %d", status)
	}
	var got *ledger.Problem
	if !errors.As(StoredProblem(WriteRecord{Response: body}), &got) {
		t.Fatal("stored problem is not a *ledger.Problem")
	}
	if !errors.Is(got, ledger.ErrRejected) || got.Reason != original.Reason || got.Detail != original.Detail || got.Posting != original.Posting {
		t.Fatalf("round trip = %+v, want %+v", got, original)
	}
	if diff := cmp.Diff(original.Results, got.Results); diff != "" {
		t.Fatalf("results (-want +got):\n%s", diff)
	}
	var unknown *ledger.Problem
	if !errors.As(StoredProblem(WriteRecord{}), &unknown) || unknown.Reason != "write-failed" {
		t.Fatalf("a record without a body must still answer a problem, got %v", unknown)
	}
	if string(CommittedBody(ledger.ID{15: 1})) != `{"id":"00000000000000000000000000000001"}` {
		t.Fatal("CommittedBody")
	}
}
