package grpcapi

import (
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

func TestEnums(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wire string
		want platformv1.HoldStatus
	}{
		{"open", platformv1.HoldStatus_HOLD_STATUS_OPEN},
		{"expired", platformv1.HoldStatus_HOLD_STATUS_EXPIRED},
		{"", platformv1.HoldStatus_HOLD_STATUS_UNSPECIFIED},
		{"nope", platformv1.HoldStatus_HOLD_STATUS_UNSPECIFIED},
	}
	for _, tt := range tests {
		if got := EnumOf[platformv1.HoldStatus](tt.wire); got != tt.want {
			t.Errorf("EnumOf(%q) = %v, want %v", tt.wire, got, tt.want)
		}
	}
	if got := ValueOf(platformv1.BalanceLimit_BALANCE_LIMIT_DEBITS_MUST_NOT_EXCEED_CREDITS); got != "debits_must_not_exceed_credits" {
		t.Errorf("ValueOf = %q", got)
	}
	if got := ValueOf(platformv1.Operation_OPERATION_UNSPECIFIED); got != "" {
		t.Errorf("ValueOf(UNSPECIFIED) = %q", got)
	}
	for _, wire := range []string{"pending", "committed", "declined", "failed"} {
		if got := ValueOf(EnumOf[platformv1.RequestStatus](wire)); got != wire {
			t.Errorf("round trip of %q gave %q", wire, got)
		}
	}
}

func TestTimes(t *testing.T) {
	t.Parallel()
	if Timestamp(time.Time{}) != nil || OptionalTimestamp(nil) != nil || TimeOf(nil) != nil {
		t.Fatal("zero and nil times must stay unset")
	}
	at := time.Date(2026, 10, 25, 12, 0, 0, 5, time.UTC)
	if got := TimeOf(Timestamp(at)); got == nil || !got.Equal(at) {
		t.Fatalf("round trip = %v, want %v", got, at)
	}
}
