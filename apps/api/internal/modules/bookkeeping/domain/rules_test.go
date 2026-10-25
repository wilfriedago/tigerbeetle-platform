package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

func TestValidatePostings(t *testing.T) {
	t.Parallel()
	ok := Posting{Source: ledger.World, Destination: "users:1:wallet", Asset: ledger.Asset{Code: "EUR", Scale: 2}, Amount: ledger.AmountFromUint64(1)}
	tests := []struct {
		name     string
		postings []Posting
		wantErr  string
	}{
		{name: "one posting", postings: []Posting{ok}},
		{name: "none", postings: nil, wantErr: "at least one"},
		{name: "same account", postings: []Posting{{Source: "a", Destination: "a", Amount: ledger.AmountFromUint64(1)}}, wantErr: "two different accounts"},
		{name: "zero amount", postings: []Posting{{Source: "a", Destination: "b"}}, wantErr: "greater than zero"},
		{name: "too many", postings: make([]Posting, MaxPostings+1), wantErr: "at most 64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePostings(tt.postings)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, ledger.ErrInvalid) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want ErrInvalid containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIdempotencyKey(t *testing.T) {
	t.Parallel()
	var p *ledger.Problem
	if err := ValidateIdempotencyKey(""); !errors.As(err, &p) || p.Reason != "missing-idempotency-key" {
		t.Fatalf("empty key: %v", err)
	}
	if err := ValidateIdempotencyKey("idem_7f3c9a12-4b0e"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIdempotencyKey("has space"); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatalf("space: %v", err)
	}
}

func TestHoldStatusAt(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 10, 24, 14, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		stored  HoldStatus
		timeout uint32
		now     time.Time
		want    HoldStatus
	}{
		{name: "open before timeout", stored: HoldOpen, timeout: 60, now: created.Add(30 * time.Second), want: HoldOpen},
		{name: "open after timeout", stored: HoldOpen, timeout: 60, now: created.Add(61 * time.Second), want: HoldExpired},
		{name: "no timeout never expires", stored: HoldOpen, timeout: 0, now: created.Add(1000 * time.Hour), want: HoldOpen},
		{name: "captured stays captured", stored: HoldCaptured, timeout: 60, now: created.Add(time.Hour), want: HoldCaptured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HoldStatusAt(tt.stored, created, tt.timeout, tt.now); got != tt.want {
				t.Fatalf("HoldStatusAt = %s, want %s", got, tt.want)
			}
		})
	}
}
