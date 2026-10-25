package ledger

import (
	"fmt"
	"math/big"
	"math/bits"
	"regexp"
)

// amountPattern is the JSON spelling of an amount (ADR-0004): no sign, no leading zeros, no
// decimals, no exponent, so two equal amounts always have one spelling.
var amountPattern = regexp.MustCompile(`^(0|[1-9]\d{0,38})$`)

// Amount is an unsigned 128-bit integer in an asset's smallest unit, the width TigerBeetle uses for
// every amount and balance. The zero value is zero. The scale lives on the asset, never here.
type Amount struct {
	hi, lo uint64
}

// MaxAmount is 2^128 - 1. TigerBeetle reads it as "the full pending amount" when posting a pending
// transfer (ADR-0010).
var MaxAmount = Amount{hi: ^uint64(0), lo: ^uint64(0)}

// maxAmountBig is MaxAmount as a big.Int, for range checks.
var maxAmountBig = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

// ParseAmount reads the decimal string form of an amount.
func ParseAmount(s string) (Amount, error) {
	if !amountPattern.MatchString(s) {
		return Amount{}, fmt.Errorf("%w: amount %q must be a non-negative integer string in the smallest unit, such as \"1250\"", ErrInvalid, s)
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return Amount{}, fmt.Errorf("%w: amount %q is not a number", ErrInvalid, s)
	}
	return AmountFromBig(v)
}

// AmountFromBig converts a big integer, failing when it does not fit in 128 bits or is negative.
func AmountFromBig(v *big.Int) (Amount, error) {
	if v.Sign() < 0 {
		return Amount{}, fmt.Errorf("%w: amount must not be negative", ErrInvalid)
	}
	if v.Cmp(maxAmountBig) > 0 {
		return Amount{}, fmt.Errorf("%w: amount exceeds 2^128 - 1", ErrInvalid)
	}
	words := v.Bits()
	var a Amount
	// big.Word is 64 bits on every platform the TigerBeetle client supports.
	if len(words) > 0 {
		a.lo = uint64(words[0])
	}
	if len(words) > 1 {
		a.hi = uint64(words[1])
	}
	return a, nil
}

// AmountFromParts builds an amount from its high and low 64-bit halves.
func AmountFromParts(hi, lo uint64) Amount {
	return Amount{hi: hi, lo: lo}
}

// AmountFromUint64 builds a small amount.
func AmountFromUint64(v uint64) Amount {
	return Amount{lo: v}
}

// Parts returns the high and low 64-bit halves.
func (a Amount) Parts() (hi, lo uint64) {
	return a.hi, a.lo
}

// Big returns the amount as a new big.Int.
func (a Amount) Big() *big.Int {
	v := new(big.Int).SetUint64(a.hi)
	v.Lsh(v, 64)
	return v.Or(v, new(big.Int).SetUint64(a.lo))
}

// String renders the decimal form used in JSON.
func (a Amount) String() string {
	if a.hi == 0 {
		return fmt.Sprintf("%d", a.lo)
	}
	return a.Big().String()
}

// IsZero reports whether the amount is zero.
func (a Amount) IsZero() bool {
	return a.hi == 0 && a.lo == 0
}

// Cmp returns -1, 0 or 1 as a is less than, equal to or greater than b.
func (a Amount) Cmp(b Amount) int {
	switch {
	case a.hi < b.hi:
		return -1
	case a.hi > b.hi:
		return 1
	case a.lo < b.lo:
		return -1
	case a.lo > b.lo:
		return 1
	default:
		return 0
	}
}

// Sub returns a - b and false when b is larger than a.
func (a Amount) Sub(b Amount) (Amount, bool) {
	lo, borrow := bits.Sub64(a.lo, b.lo, 0)
	hi, borrow := bits.Sub64(a.hi, b.hi, borrow)
	if borrow != 0 {
		return Amount{}, false
	}
	return Amount{hi: hi, lo: lo}, true
}

// MarshalText implements encoding.TextMarshaler so amounts are JSON strings, never numbers.
func (a Amount) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (a *Amount) UnmarshalText(text []byte) error {
	v, err := ParseAmount(string(text))
	if err != nil {
		return err
	}
	*a = v
	return nil
}
