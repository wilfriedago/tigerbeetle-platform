package kafka

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// TestRecordOf pins the JSON of a ledger event: every u128 and u64 a string, IDs in hex, unknown
// facts null.
func TestRecordOf(t *testing.T) {
	t.Parallel()
	big, err := ledger.ParseAmount("340282366920938463463374607431768211455")
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := ledger.ParseID("0192f3a1b7c94e2a8d5f6c0b1e2d7c2e")
	leg, _ := ledger.ParseID("0192f3a1b7c94e2a8d5f6c0b1e2d7c2f")
	ev := domain.LedgerEvent{
		ChangeEvent: domain.ChangeEvent{
			Type: domain.ChangeTwoPhasePending, Timestamp: 1792967077287893519, TBLedger: 1,
			Transfer: domain.ChangeTransfer{ID: leg, Amount: ledger.AmountFromUint64(200000), UserData128: tx, UserData64: 1 << 60, Timeout: 2400, Code: 7, Flags: 2, Timestamp: 1792967077287893519},
			Debit:    domain.ChangeAccount{ID: ledger.ID{15: 1}, CreditsPosted: big, Code: 10},
			Credit:   domain.ChangeAccount{ID: ledger.ID{15: 2}, Code: 20},
		},
		Ledger: "eu", Asset: "EUR/2", DebitAddress: "users:1:wallet",
	}
	raw, err := json.Marshal(RecordOf(&ev))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"type": "two_phase_pending", "timestamp": "1792967077287893519", "ledger": "eu", "asset": "EUR/2", "tbLedger": float64(1),
		"transactionId": "0192f3a1b7c94e2a8d5f6c0b1e2d7c2e",
		"transfer": map[string]any{
			"id": "0192f3a1b7c94e2a8d5f6c0b1e2d7c2f", "amount": "200000", "pendingId": nil,
			"userData128": "0192f3a1b7c94e2a8d5f6c0b1e2d7c2e", "userData64": "1152921504606846976", "userData32": float64(0),
			"timeout": float64(2400), "code": float64(7), "flags": float64(2), "timestamp": "1792967077287893519",
		},
		"debitAccount": map[string]any{
			"id": "00000000000000000000000000000001", "address": "users:1:wallet",
			"debitsPending": "0", "debitsPosted": "0", "creditsPending": "0", "creditsPosted": "340282366920938463463374607431768211455",
			"userData128": "00000000000000000000000000000000", "userData64": "0", "userData32": float64(0),
			"code": float64(10), "flags": float64(0), "timestamp": "0",
		},
		"creditAccount": map[string]any{
			"id": "00000000000000000000000000000002", "address": nil,
			"debitsPending": "0", "debitsPosted": "0", "creditsPending": "0", "creditsPosted": "0",
			"userData128": "00000000000000000000000000000000", "userData64": "0", "userData32": float64(0),
			"code": float64(20), "flags": float64(0), "timestamp": "0",
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("record (-want +got):\n%s", diff)
	}
}
