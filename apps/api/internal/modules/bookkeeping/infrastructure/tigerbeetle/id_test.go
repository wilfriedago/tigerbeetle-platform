package tigerbeetle

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	tb "github.com/tigerbeetle/tigerbeetle-go"
)

func TestIDConversion(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()
		original := tb.ID()
		if got := FromDomainID(ToDomainID(original)); got != original {
			t.Fatalf("round trip changed the id: %s != %s", got, original)
		}
	})

	t.Run("string form matches the client", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name string
			id   tb.Uint128
		}{
			{name: "small", id: tb.ToUint128(1)},
			{name: "max uint64", id: tb.ToUint128(^uint64(0))},
			{name: "generated", id: tb.ID()},
		}
		for _, tt := range tests {
			want := tt.id.String() // hexadecimal without leading zeroes
			got := strings.TrimLeft(ToDomainID(tt.id).String(), "0")
			if got != want {
				t.Errorf("%s: domain id %q, client id %q", tt.name, got, want)
			}
		}
	})

	t.Run("generated ids keep time order as bytes", func(t *testing.T) {
		t.Parallel()
		first := ToDomainID(tb.ID())
		second := ToDomainID(tb.ID())
		if bytes.Compare(first[:], second[:]) >= 0 {
			t.Fatalf("expected %s < %s", first, second)
		}
	})

	t.Run("value one lands in the last byte", func(t *testing.T) {
		t.Parallel()
		id := ToDomainID(tb.ToUint128(1))
		if want := fmt.Sprintf("%031d1", 0); id.String() != want {
			t.Fatalf("got %s, want %s", id, want)
		}
	})
}

func TestNewIDIsTimeOrdered(t *testing.T) {
	t.Parallel()
	var s Store
	first, second := s.NewID(), s.NewID()
	if first.IsZero() || second.IsZero() {
		t.Fatal("NewID returned zero")
	}
	if bytes.Compare(first[:], second[:]) >= 0 {
		t.Fatalf("NewID is not increasing: %s then %s", first, second)
	}
}
