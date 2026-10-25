package scalar

import (
	"math/big"
	"strconv"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Conversions from kernel values to their wire form, shared by every transport.

// MetadataOf answers {} rather than null for metadata never set.
func MetadataOf(m map[string]string) Metadata {
	if m == nil {
		return Metadata{}
	}
	return m
}

// Map passes {} rather than nil to the use cases for metadata absent from a request.
func (m Metadata) Map() map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// AssetsOf answers [] rather than null for no assets.
func AssetsOf(assets []string) []Asset {
	out := make([]Asset, len(assets))
	for i, a := range assets {
		out[i] = Asset(a)
	}
	return out
}

// IDOf renders an ID as 32 lowercase hexadecimal characters.
func IDOf(id ledger.ID) ID {
	return ID(id.String())
}

// OptionalID renders an optional ID; nil stays absent.
func OptionalID(id *ledger.ID) *ID {
	if id == nil {
		return nil
	}
	s := IDOf(*id)
	return &s
}

// AmountOf renders an amount exactly.
func AmountOf(a ledger.Amount) Amount {
	return Amount(a.String())
}

// Signed renders a balance exactly, never through a float (ADR-0004). nil is zero.
func Signed(v *big.Int) SignedAmount {
	if v == nil {
		return "0"
	}
	return SignedAmount(v.String())
}

// TimestampOf renders a TigerBeetle timestamp as a decimal string.
func TimestampOf(ts uint64) Timestamp {
	return Timestamp(strconv.FormatUint(ts, 10))
}
