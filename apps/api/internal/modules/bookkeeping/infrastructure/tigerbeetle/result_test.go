package tigerbeetle

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	tb "github.com/tigerbeetle/tigerbeetle-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

func TestResultName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status tb.CreateTransferStatus
		want   string
	}{
		{status: tb.TransferCreated, want: "created"},
		{status: tb.TransferExists, want: "exists"},
		{status: tb.TransferExceedsCredits, want: "exceeds_credits"},
		{status: tb.TransferLinkedEventFailed, want: "linked_event_failed"},
		{status: tb.TransferIDAlreadyFailed, want: "id_already_failed"},
		{status: tb.TransferDebitAccountIDMustNotBeZero, want: "debit_account_id_must_not_be_zero"},
		{status: tb.TransferPendingTransferExpired, want: "pending_transfer_expired"},
		{status: tb.TransferExceedsPendingTransferAmount, want: "exceeds_pending_transfer_amount"},
	}
	for _, tt := range tests {
		if got := resultName(tt.status.String(), "Transfer"); got != tt.want {
			t.Errorf("resultName(%s) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestAmountRoundTrip(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"0", "1250", "18446744073709551616", "340282366920938463463374607431768211455"} {
		a, err := ledger.ParseAmount(s)
		if err != nil {
			t.Fatal(err)
		}
		u := uint128Of(a)
		if bi := u.BigInt(); bi.String() != s {
			t.Errorf("uint128Of(%s) = %s", s, bi.String())
		}
		if got := amountOf(u); got.Cmp(a) != 0 {
			t.Errorf("amountOf(uint128Of(%s)) = %s", s, got)
		}
	}
}

// TestResultReasonsFollowAIP193 walks every transfer result the client knows: a declined chain's
// problem type is derived from its result, and gRPC reports the type's slug as an AIP-193
// ErrorInfo reason (plan section 9.2), so each one must fit the pattern.
func TestResultReasonsFollowAIP193(t *testing.T) {
	t.Parallel()
	statuses := []tb.CreateTransferStatus{tb.TransferCreated}
	for code := range tb.CreateTransferStatus(256) {
		statuses = append(statuses, code)
	}
	seen := 0
	for _, status := range statuses {
		name := status.String()
		if strings.HasPrefix(name, "CreateTransferStatus(") {
			continue
		}
		seen++
		reason := problem.ErrorReasonFor(domain.ResultReason(resultName(name, "Transfer")))
		if !problem.ValidErrorReason(reason) {
			t.Errorf("%s gives ErrorInfo reason %q, which breaks AIP-193", name, reason)
		}
	}
	if seen < 50 {
		t.Fatalf("walked %d named results, want the client's full list", seen)
	}
}

// TestTransferResultsMatchTheClient keeps domain.TransferResults, which the problem catalogue
// documents without linking the cgo client, equal to the results the client knows.
func TestTransferResultsMatchTheClient(t *testing.T) {
	t.Parallel()
	var got []string
	for code := range tb.CreateTransferStatus(256) {
		name := code.String()
		if strings.HasPrefix(name, "CreateTransferStatus(") {
			continue
		}
		got = append(got, resultName(name, "Transfer"))
	}
	if diff := cmp.Diff(domain.TransferResults, got); diff != "" {
		t.Fatalf("domain.TransferResults differs from the client (-listed +client):\n%s", diff)
	}
}
