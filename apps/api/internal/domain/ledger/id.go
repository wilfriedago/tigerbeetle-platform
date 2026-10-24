package ledger

import (
	"encoding/hex"
	"fmt"
)

// ID is a 128-bit identifier in big-endian byte order, so that comparing the bytes compares the
// numbers and time-ordered IDs stay time-ordered when stored in a Postgres uuid column
// (ADR-0002). TigerBeetle's client uses little-endian bytes; the tigerbeetle adapter converts.
type ID [16]byte

// ParseID reads the 32-character hexadecimal form produced by String.
func ParseID(s string) (ID, error) {
	var id ID
	if len(s) != 32 {
		return id, fmt.Errorf("%w: id must be 32 hexadecimal characters", ErrInvalid)
	}
	if _, err := hex.Decode(id[:], []byte(s)); err != nil {
		return id, fmt.Errorf("%w: id must be hexadecimal", ErrInvalid)
	}
	return id, nil
}

// String renders the ID as 32 lowercase hexadecimal characters, most significant byte first.
func (id ID) String() string {
	return hex.EncodeToString(id[:])
}

// IsZero reports whether the ID is all zeroes, which TigerBeetle never accepts as an identifier.
func (id ID) IsZero() bool {
	return id == ID{}
}
