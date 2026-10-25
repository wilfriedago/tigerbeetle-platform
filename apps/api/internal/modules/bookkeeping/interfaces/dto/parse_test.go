package dto

import (
	"errors"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func TestParsePostings(t *testing.T) {
	good := Posting{Source: "world", Destination: "users:1:wallet", Asset: "EUR/2", Amount: "100"}
	tests := []struct {
		name        string
		in          []Posting
		wantPosting int
		wantErr     bool
	}{
		{name: "valid", in: []Posting{good, good}},
		{name: "bad amount names the posting", in: []Posting{good, {Source: "world", Destination: "a", Asset: "EUR/2", Amount: "-1"}}, wantPosting: 1, wantErr: true},
		{name: "bad address", in: []Posting{{Source: "a b", Destination: "a", Asset: "EUR/2", Amount: "1"}}, wantPosting: 0, wantErr: true},
		{name: "bad asset", in: []Posting{{Source: "a", Destination: "b", Asset: "EUR/x", Amount: "1"}}, wantPosting: 0, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePostings(tt.in)
			if !tt.wantErr {
				if err != nil || len(got) != len(tt.in) {
					t.Fatalf("ParsePostings = %v, %v", got, err)
				}
				return
			}
			var p *ledger.Problem
			if !errors.As(err, &p) || !errors.Is(err, ledger.ErrInvalid) {
				t.Fatalf("err = %v, want an invalid-request problem", err)
			}
			if p.Posting != tt.wantPosting || p.Reason != "invalid-request" {
				t.Errorf("posting %d reason %q, want posting %d", p.Posting, p.Reason, tt.wantPosting)
			}
		})
	}
}
