package problem

import (
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func TestErrorReason(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		p    Problem
		want string
	}{
		{name: "typed", p: From(ledger.NewProblem(ledger.ErrRejected, "exceeds-credits", "")), want: "EXCEEDS_CREDITS"},
		{name: "sentinel", p: From(ledger.ErrNotFound), want: "NOT_FOUND"},
		{name: "blank uses the kind", p: Blank(500, "Internal server error"), want: "INTERNAL"},
		{name: "blank not found", p: Blank(404, "Not Found"), want: "NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.p.ErrorReason(); got != tt.want {
				t.Fatalf("ErrorReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestKnownReasonsFollowAIP193 fails the build when a new problem type has a slug that gRPC
// clients could not use as an ErrorInfo reason (plan section 9.2, decided R3).
func TestKnownReasonsFollowAIP193(t *testing.T) {
	t.Parallel()
	reasons := KnownReasons()
	if len(reasons) < len(titles) {
		t.Fatalf("KnownReasons() = %d reasons, want at least %d", len(reasons), len(titles))
	}
	for _, reason := range reasons {
		got := ErrorReasonOf(TypeOf(reason))
		if !ValidErrorReason(got) {
			t.Errorf("reason %q gives ErrorInfo reason %q, which breaks AIP-193", reason, got)
		}
	}
	for _, kind := range []Kind{KindInvalid, KindNotFound, KindAlreadyExists, KindConflict, KindRejected, KindUnavailable, KindInternal} {
		if !ValidErrorReason(string(kind)) {
			t.Errorf("kind %q breaks AIP-193", kind)
		}
	}
}

func TestValidErrorReason(t *testing.T) {
	t.Parallel()
	for reason, want := range map[string]bool{
		"EXCEEDS_CREDITS": true,
		"AB1":             true,
		"A1":              false,
		"exceeds_credits": false,
		"_LEADING":        false,
		"TRAILING_":       false,
		"X":               false,
		"WAY_TOO_LONG_" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA": false,
	} {
		if got := ValidErrorReason(reason); got != want {
			t.Errorf("ValidErrorReason(%q) = %v, want %v", reason, got, want)
		}
	}
}
