package app

import (
	"bytes"
	"errors"
	"testing"
	"time"

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
		verdict verdict
		failed  int
		result  string
	}{
		{name: "all created", results: []string{ResultCreated, ResultCreated}, verdict: verdictCommitted, failed: -1},
		{name: "already landed", results: []string{ResultExists, ResultLinkedEventFailed, ResultLinkedEventFailed}, verdict: verdictCommitted, failed: -1},
		{name: "every transfer exists", results: []string{ResultExists, ResultExists}, verdict: verdictCommitted, failed: -1},
		{name: "second posting declined", results: []string{ResultLinkedEventFailed, ResultExceedsCredits, ResultLinkedEventFailed}, verdict: verdictDeclined, failed: 1, result: ResultExceedsCredits},
		{name: "one posting declined", results: []string{ResultExceedsDebits}, verdict: verdictDeclined, failed: 0, result: ResultExceedsDebits},
		{name: "burned id", results: []string{ResultIDAlreadyFailed, ResultLinkedEventFailed}, verdict: verdictBurned, failed: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classify(outcomes(tt.results...))
			if got.verdict != tt.verdict || got.failed != tt.failed || got.result != tt.result {
				t.Fatalf("classify(%v) = %+v, want verdict %d failed %d result %q", tt.results, got, tt.verdict, tt.failed, tt.result)
			}
			if tt.verdict == verdictCommitted && tt.results[0] == ResultCreated && got.timestamp != uint64(100+len(tt.results)-1) {
				t.Fatalf("timestamp = %d, want the latest", got.timestamp)
			}
		})
	}
}

func TestDeclineProblemNamesThePosting(t *testing.T) {
	t.Parallel()
	eur := ledger.Asset{Code: "EUR", Scale: 2}
	postings := []ledger.Posting{
		{Source: ledger.World, Destination: "users:1:wallet", Asset: eur, Amount: ledger.AmountFromUint64(1)},
		{Source: "users:1:wallet", Destination: "merchants:42:main", Asset: eur, Amount: ledger.AmountFromUint64(9)},
	}
	p := declineProblem(classify(outcomes(ResultLinkedEventFailed, ResultExceedsCredits)), postings, nil)
	if p.Reason != "exceeds-credits" || p.Posting != 1 || !errors.Is(p, ledger.ErrRejected) {
		t.Fatalf("problem = %+v", p)
	}
	want := "Posting 2 would take users:1:wallet below zero in EUR/2. Nothing was applied: the postings are linked."
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func TestStoredProblemRoundTrip(t *testing.T) {
	t.Parallel()
	original := &ledger.Problem{Kind: ledger.ErrRejected, Reason: "exceeds-credits", Detail: "d", Posting: 0, Results: []string{"exceeds_credits"}}
	status, body := problemOutcome(original)
	if status != statusRejected {
		t.Fatalf("status = %d", status)
	}
	var got *ledger.Problem
	if !errors.As(storedProblem(WriteRecord{Response: body}), &got) {
		t.Fatal("stored problem is not a *ledger.Problem")
	}
	if !errors.Is(got, ledger.ErrRejected) || got.Reason != original.Reason || got.Detail != original.Detail || got.Posting != original.Posting {
		t.Fatalf("round trip = %+v, want %+v", got, original)
	}
	if diff := cmp.Diff(original.Results, got.Results); diff != "" {
		t.Fatalf("results (-want +got):\n%s", diff)
	}
	var unknown *ledger.Problem
	if !errors.As(storedProblem(WriteRecord{}), &unknown) || unknown.Reason != "write-failed" {
		t.Fatalf("a record without a body must still answer a problem, got %v", unknown)
	}
}

func TestCanonicalRequestIsStable(t *testing.T) {
	t.Parallel()
	a := transactionRequest{Postings: []postingRequest{{Source: "world", Destination: "users:1", Asset: "EUR/2", Amount: "1"}}, Metadata: map[string]string{"b": "2", "a": "1"}}
	b := transactionRequest{Postings: []postingRequest{{Source: "world", Destination: "users:1", Asset: "EUR/2", Amount: "1"}}, Metadata: map[string]string{"a": "1", "b": "2"}}
	_, ha, err := canonicalRequest(a)
	if err != nil {
		t.Fatal(err)
	}
	_, hb, _ := canonicalRequest(b)
	if !bytes.Equal(ha, hb) {
		t.Fatal("equal requests hashed differently")
	}
	b.Postings[0].Amount = "2"
	_, hc, _ := canonicalRequest(b)
	if bytes.Equal(ha, hc) {
		t.Fatal("different amounts hashed equally")
	}
}

func TestWriteCursorRoundTrip(t *testing.T) {
	t.Parallel()
	c := WriteCursor{CreatedAt: time.Date(2026, 10, 24, 14, 32, 8, 123456000, time.UTC), Key: "idem|with|pipes"}
	got, err := decodeWriteCursor(encodeWriteCursor(c))
	if err != nil {
		t.Fatal(err)
	}
	if !got.CreatedAt.Equal(c.CreatedAt) || got.Key != c.Key {
		t.Fatalf("round trip = %+v, want %+v", got, c)
	}
	if _, err := decodeIDCursor(encodeCursor("not-an-id")); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatalf("bad ID cursor error = %v", err)
	}
}
