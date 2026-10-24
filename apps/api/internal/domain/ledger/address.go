package ledger

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxAddressLength bounds account addresses. Addresses are keys in Postgres only; TigerBeetle
// accounts are identified by 128-bit IDs (ADR-0006).
const MaxAddressLength = 255

// World is the conventional counterpart for money entering or leaving the ledger. It has no
// balance limit (ADR-0006).
const World Address = "world"

var addressPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+(?::[a-zA-Z0-9_-]+)*$`)

// Address is a hierarchical account name such as "users:1234:wallet" (ADR-0004). Segments are
// separated by colons; prefixes such as "users:" drive aggregated balances and account rules.
type Address string

// ParseAddress validates an address.
func ParseAddress(s string) (Address, error) {
	if len(s) > MaxAddressLength {
		return "", fmt.Errorf("%w: address longer than %d characters", ErrInvalid, MaxAddressLength)
	}
	if !addressPattern.MatchString(s) {
		return "", fmt.Errorf("%w: address %q must be colon-separated segments of [a-zA-Z0-9_-]", ErrInvalid, s)
	}
	return Address(s), nil
}

// Segments splits the address on colons.
func (a Address) Segments() []string {
	return strings.Split(string(a), ":")
}

// HasPrefix reports whether the address lies under the given segment prefix. The prefix matches
// whole segments only, so "users:1" does not match "users:12:wallet".
func (a Address) HasPrefix(prefix Address) bool {
	if a == prefix {
		return true
	}
	return strings.HasPrefix(string(a), string(prefix)+":")
}
