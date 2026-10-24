package tigerbeetle

import (
	tb "github.com/tigerbeetle/tigerbeetle-go"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// ToDomainID converts a TigerBeetle identifier to the domain representation. The client stores
// its 128-bit numbers little-endian (least significant byte first); the domain and Postgres uuid
// columns use big-endian so byte order equals numeric order (ADR-0002).
func ToDomainID(id tb.Uint128) ledger.ID {
	return ledger.ID(reverse(id.Bytes()))
}

// FromDomainID converts a domain identifier back to the client representation.
func FromDomainID(id ledger.ID) tb.Uint128 {
	return tb.BytesToUint128(reverse(id))
}

func reverse(b [16]byte) [16]byte {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}
