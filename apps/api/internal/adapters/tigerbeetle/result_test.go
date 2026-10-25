package tigerbeetle

import (
	"testing"

	tb "github.com/tigerbeetle/tigerbeetle-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
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
