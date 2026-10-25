package ledger

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValidateLedgerName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple", input: "main"},
		{name: "with separators", input: "acme_prod-eu"},
		{name: "starts with digit", input: "1st"},
		{name: "empty", input: "", wantErr: true},
		{name: "uppercase", input: "Main", wantErr: true},
		{name: "starts with dash", input: "-main", wantErr: true},
		{name: "too long", input: string(make([]byte, 64)), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateLedgerName(tt.input)
			if tt.wantErr != (err != nil) {
				t.Fatalf("ValidateLedgerName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Fatalf("error %v is not ErrInvalid", err)
			}
		})
	}
}

func TestParseAsset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    Asset
		wantStr string
		wantErr bool
	}{
		{name: "zero scale", input: "XOF/0", want: Asset{Code: "XOF"}, wantStr: "XOF/0"},
		{name: "two decimals", input: "EUR/2", want: Asset{Code: "EUR", Scale: 2}, wantStr: "EUR/2"},
		{name: "implicit scale", input: "BTC", want: Asset{Code: "BTC"}, wantStr: "BTC/0"},
		{name: "max scale", input: "ETH/18", want: Asset{Code: "ETH", Scale: 18}, wantStr: "ETH/18"},
		{name: "scale too large", input: "ETH/19", wantErr: true},
		{name: "lowercase", input: "eur/2", wantErr: true},
		{name: "single letter", input: "E/2", wantErr: true},
		{name: "negative scale", input: "EUR/-1", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAsset(tt.input)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("ParseAsset(%q) error = %v, want ErrInvalid", tt.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAsset(%q) unexpected error: %v", tt.input, err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("ParseAsset(%q) mismatch (-want +got):\n%s", tt.input, diff)
			}
			if got.String() != tt.wantStr {
				t.Fatalf("String() = %q, want %q", got.String(), tt.wantStr)
			}
		})
	}
}

func TestParseAddress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "world", input: "world"},
		{name: "hierarchical", input: "users:1234:wallet"},
		{name: "dashes and underscores", input: "bank-accounts:ab_12"},
		{name: "empty", input: "", wantErr: true},
		{name: "empty segment", input: "users::wallet", wantErr: true},
		{name: "trailing colon", input: "users:", wantErr: true},
		{name: "space", input: "users 1", wantErr: true},
		{name: "too long", input: string(make([]byte, 256)), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseAddress(tt.input)
			if tt.wantErr != (err != nil) {
				t.Fatalf("ParseAddress(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestAddressHasPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		addr, prefix Address
		want         bool
	}{
		{addr: "users:1234:wallet", prefix: "users", want: true},
		{addr: "users:1234:wallet", prefix: "users:1234", want: true},
		{addr: "users:1234:wallet", prefix: "users:1234:wallet", want: true},
		{addr: "users:12:wallet", prefix: "users:1", want: false},
		{addr: "world", prefix: "users", want: false},
	}
	for _, tt := range tests {
		if got := tt.addr.HasPrefix(tt.prefix); got != tt.want {
			t.Errorf("%q.HasPrefix(%q) = %v, want %v", tt.addr, tt.prefix, got, tt.want)
		}
	}
}

func TestIDRoundTrip(t *testing.T) {
	t.Parallel()
	var id ID
	for i := range id {
		id[i] = byte(i)
	}
	got, err := ParseID(id.String())
	if err != nil {
		t.Fatalf("ParseID: %v", err)
	}
	if got != id {
		t.Fatalf("round trip mismatch: %s != %s", got, id)
	}
	if _, err := ParseID("abc"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short id error = %v, want ErrInvalid", err)
	}
	if !(ID{}).IsZero() || id.IsZero() {
		t.Fatal("IsZero mismatch")
	}
}
