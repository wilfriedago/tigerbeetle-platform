package scalar

import (
	"math/big"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func TestConversions(t *testing.T) {
	t.Parallel()
	if got := MetadataOf(nil); got == nil || len(got) != 0 {
		t.Errorf("MetadataOf(nil) = %#v", got)
	}
	if got := Metadata(nil).Map(); got == nil || len(got) != 0 {
		t.Errorf("Metadata(nil).Map() = %#v", got)
	}
	if diff := cmp.Diff([]Asset{"EUR/2"}, AssetsOf([]string{"EUR/2"})); diff != "" {
		t.Error(diff)
	}
	if got := AssetsOf(nil); got == nil {
		t.Error("AssetsOf(nil) is nil, want empty")
	}
	id := ledger.ID{15: 1}
	if got := IDOf(id); got != "00000000000000000000000000000001" {
		t.Errorf("IDOf = %q", got)
	}
	if OptionalID(nil) != nil || *OptionalID(&id) != IDOf(id) {
		t.Error("OptionalID")
	}
	if Signed(nil) != "0" || Signed(big.NewInt(-12)) != "-12" {
		t.Error("Signed")
	}
	if TimestampOf(1745000000000000000) != "1745000000000000000" {
		t.Error("TimestampOf")
	}
	if AmountOf(ledger.AmountFromUint64(1250)) != "1250" {
		t.Error("AmountOf")
	}
}
