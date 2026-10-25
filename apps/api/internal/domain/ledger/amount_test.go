package ledger

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestParseAmount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "0", want: "0"},
		{input: "1250", want: "1250"},
		{input: "18446744073709551615", want: "18446744073709551615"},
		{input: "18446744073709551616", want: "18446744073709551616"},
		{input: "340282366920938463463374607431768211455", want: "340282366920938463463374607431768211455"},
		{input: "340282366920938463463374607431768211456", wantErr: true},
		{input: "-1", wantErr: true},
		{input: "01", wantErr: true},
		{input: "1.5", wantErr: true},
		{input: "1e3", wantErr: true},
		{input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAmount(tt.input)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("ParseAmount(%q) error = %v, want ErrInvalid", tt.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAmount(%q): %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Fatalf("ParseAmount(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestAmountArithmetic(t *testing.T) {
	t.Parallel()
	big := AmountFromParts(1, 5)
	small := AmountFromUint64(10)
	if big.Cmp(small) != 1 || small.Cmp(big) != -1 || big.Cmp(AmountFromParts(1, 5)) != 0 {
		t.Fatal("Cmp disagrees with the ordering of the halves")
	}
	diff, ok := big.Sub(small)
	if !ok || diff.String() != "18446744073709551611" {
		t.Fatalf("Sub = %s, %v; want 18446744073709551611, true", diff, ok)
	}
	if _, ok := small.Sub(big); ok {
		t.Fatal("Sub underflow reported no borrow")
	}
	if MaxAmount.String() != "340282366920938463463374607431768211455" {
		t.Fatalf("MaxAmount = %s", MaxAmount)
	}
}

func TestAmountJSONIsAString(t *testing.T) {
	t.Parallel()
	out, err := json.Marshal(struct{ A Amount }{A: AmountFromUint64(1250)})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"A":"1250"}` {
		t.Fatalf("marshal = %s", out)
	}
	var in struct{ A Amount }
	if err := json.Unmarshal([]byte(`{"A":"42"}`), &in); err != nil || in.A.String() != "42" {
		t.Fatalf("unmarshal = %v, %v", in.A, err)
	}
}

func TestHoldStatusAt(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 10, 24, 14, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		stored  HoldStatus
		timeout uint32
		now     time.Time
		want    HoldStatus
	}{
		{name: "open before timeout", stored: HoldOpen, timeout: 60, now: created.Add(30 * time.Second), want: HoldOpen},
		{name: "open after timeout", stored: HoldOpen, timeout: 60, now: created.Add(61 * time.Second), want: HoldExpired},
		{name: "no timeout never expires", stored: HoldOpen, timeout: 0, now: created.Add(1000 * time.Hour), want: HoldOpen},
		{name: "captured stays captured", stored: HoldCaptured, timeout: 60, now: created.Add(time.Hour), want: HoldCaptured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HoldStatusAt(tt.stored, created, tt.timeout, tt.now); got != tt.want {
				t.Fatalf("HoldStatusAt = %s, want %s", got, tt.want)
			}
		})
	}
}
