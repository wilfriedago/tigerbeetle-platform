package cdc

import (
	"strings"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// sample is a message the CDC job of TigerBeetle 0.17.9 published in spike S3 (2026-10-25),
// unchanged: u128 IDs as strings, small u128 and u64 values as numbers.
const sample = `{"timestamp":"1792967077287893519","type":"two_phase_pending","ledger":1,` +
	`"transfer":{"id":"2167564193439630638956872914678427098","amount":200000,"pending_id":0,` +
	`"user_data_128":"2167564193439630638956872914678427097","user_data_64":0,"user_data_32":0,` +
	`"timeout":2400,"code":7,"flags":2,"timestamp":"1792967077287893519"},` +
	`"debit_account":{"id":"2167564192639400415754287318891559133","debits_pending":200000,` +
	`"debits_posted":125000,"credits_pending":0,"credits_posted":500000,"user_data_128":0,` +
	`"user_data_64":0,"user_data_32":0,"code":10,"flags":10,"timestamp":"1792967076622734816"},` +
	`"credit_account":{"id":"2167564193038870995461052280263025099","debits_pending":0,` +
	`"debits_posted":100000,"credits_pending":200000,"credits_posted":128990,"user_data_128":0,` +
	`"user_data_64":0,"user_data_32":0,"code":20,"flags":2,"timestamp":"1792967076949198708"}}`

func TestDecodeSample(t *testing.T) {
	t.Parallel()
	ev, err := Decode([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Type != domain.ChangeTwoPhasePending || ev.Timestamp != 1792967077287893519 || ev.TBLedger != 1 {
		t.Fatalf("event = %+v", ev)
	}
	// The decimal u128 becomes the 32-hex ID the API prints for the same number.
	if got := ev.Transfer.ID.String(); got != "01a1752a6d9d672dd3c85aaaa691adda" {
		t.Fatalf("transfer ID = %s, want 01a1752a6d9d672dd3c85aaaa691adda", got)
	}
	if ev.Transfer.Amount.String() != "200000" || ev.Transfer.Timeout != 2400 || ev.Transfer.Code != 7 {
		t.Errorf("transfer = %+v", ev.Transfer)
	}
	if ev.Debit.DebitsPending.String() != "200000" || ev.Debit.CreditsPosted.String() != "500000" || ev.Credit.Code != 20 {
		t.Errorf("accounts = %+v / %+v", ev.Debit, ev.Credit)
	}
}

func TestDecodeIntegers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		amount string
		want   string
		bad    bool
	}{
		{name: "largest exact JSON number", amount: `9007199254740991`, want: "9007199254740991"},
		{name: "first string", amount: `"9007199254740992"`, want: "9007199254740992"},
		{name: "a number above 2^53 is still exact", amount: `9007199254740993`, want: "9007199254740993"},
		{name: "u128 max", amount: `"340282366920938463463374607431768211455"`, want: "340282366920938463463374607431768211455"},
		{name: "above u128", amount: `"340282366920938463463374607431768211456"`, bad: true},
		{name: "negative", amount: `-1`, bad: true},
		{name: "fraction", amount: `1.5`, bad: true},
		{name: "exponent", amount: `1e3`, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := strings.Replace(sample, `"amount":200000`, `"amount":`+tt.amount, 1)
			ev, err := Decode([]byte(body))
			if tt.bad {
				if err == nil {
					t.Fatalf("Decode accepted %s", tt.amount)
				}
				return
			}
			if err != nil || ev.Transfer.Amount.String() != tt.want {
				t.Fatalf("amount = %s, %v; want %s", ev.Transfer.Amount, err, tt.want)
			}
		})
	}
}

func TestDecodeRefusesUnknownTypes(t *testing.T) {
	t.Parallel()
	if _, err := Decode([]byte(strings.Replace(sample, "two_phase_pending", "mystery", 1))); err == nil {
		t.Fatal("Decode accepted an unknown event type")
	}
	if _, err := Decode([]byte(`{`)); err == nil {
		t.Fatal("Decode accepted broken JSON")
	}
}
