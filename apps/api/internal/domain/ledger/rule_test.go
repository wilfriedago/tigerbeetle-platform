package ledger

import (
	"errors"
	"strings"
	"testing"
)

func TestParsePattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		segments int
		prefix   bool
		wantErr  bool
	}{
		{input: "*", segments: 0},
		{input: "world", segments: 1},
		{input: "users:", segments: 1, prefix: true},
		{input: "treasury:fx:", segments: 2, prefix: true},
		{input: "users:1234:wallet", segments: 3},
		{input: "", wantErr: true},
		{input: "users::", wantErr: true},
		{input: "**", wantErr: true},
		{input: "users:*", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			p, err := ParsePattern(tt.input)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("ParsePattern(%q) error = %v, want ErrInvalid", tt.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePattern(%q): %v", tt.input, err)
			}
			if p.Segments() != tt.segments || p.IsPrefix() != tt.prefix {
				t.Fatalf("%q: segments %d prefix %v, want %d %v", tt.input, p.Segments(), p.IsPrefix(), tt.segments, tt.prefix)
			}
		})
	}
}

func TestPatternMatchesWholeSegments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern Pattern
		address Address
		want    bool
	}{
		{pattern: "*", address: "anything:at:all", want: true},
		{pattern: "users:", address: "users:1234:wallet", want: true},
		{pattern: "users:1:", address: "users:1:wallet", want: true},
		{pattern: "users:1:", address: "users:12:wallet", want: false},
		{pattern: "users:", address: "users", want: false},
		{pattern: "world", address: "world", want: true},
		{pattern: "world", address: "world:sub", want: false},
	}
	for _, tt := range tests {
		if got := tt.pattern.Matches(tt.address); got != tt.want {
			t.Errorf("%q.Matches(%q) = %v, want %v", tt.pattern, tt.address, got, tt.want)
		}
	}
}

func TestMatchRuleMostSpecificWins(t *testing.T) {
	t.Parallel()
	stored := []Rule{
		{Pattern: "users:", Limit: LimitDebitsMustNotExceedCredits, Code: 10},
		{Pattern: "treasury:fx:", Limit: LimitNone, History: true, Code: 30},
		{Pattern: "treasury:", Limit: LimitDebitsMustNotExceedCredits, Code: 31},
	}
	tests := []struct {
		address Address
		want    Pattern
	}{
		{address: "treasury:fx:eur", want: "treasury:fx:"},
		{address: "treasury:ops", want: "treasury:"},
		{address: "users:1234:wallet", want: "users:"},
		{address: "world", want: "world"},
		{address: "merchants:42:main", want: "*"},
	}
	for _, tt := range tests {
		got, candidates := MatchRule(stored, tt.address)
		if got.Pattern != tt.want {
			t.Errorf("MatchRule(%q) = %q, want %q", tt.address, got.Pattern, tt.want)
		}
		if len(candidates) != 5 {
			t.Errorf("MatchRule(%q) returned %d candidates, want the 3 stored rules and 2 defaults", tt.address, len(candidates))
		}
	}
}

func TestStoredRuleOverridesDefault(t *testing.T) {
	t.Parallel()
	got, _ := MatchRule([]Rule{{Pattern: "*", Limit: LimitNone, Code: 9}}, "users:1")
	if got.BuiltIn || got.Limit != LimitNone || got.Code != 9 {
		t.Fatalf("stored catch-all did not replace the default: %+v", got)
	}
}

func TestValidatePostings(t *testing.T) {
	t.Parallel()
	ok := Posting{Source: World, Destination: "users:1:wallet", Asset: Asset{Code: "EUR", Scale: 2}, Amount: AmountFromUint64(1)}
	tests := []struct {
		name     string
		postings []Posting
		wantErr  string
	}{
		{name: "one posting", postings: []Posting{ok}},
		{name: "none", postings: nil, wantErr: "at least one"},
		{name: "same account", postings: []Posting{{Source: "a", Destination: "a", Amount: AmountFromUint64(1)}}, wantErr: "two different accounts"},
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
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want ErrInvalid containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMetadata(t *testing.T) {
	t.Parallel()
	if err := ValidateMetadata(map[string]string{"order_id": "ord_8812", "region": "eu-west"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMetadata(map[string]string{"bad key": "x"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("space in key: %v", err)
	}
	if err := ValidateMetadata(map[string]string{"k": strings.Repeat("v", MaxMetadataValueLength+1)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("long value: %v", err)
	}
}

func TestValidateIdempotencyKey(t *testing.T) {
	t.Parallel()
	var p *Problem
	if err := ValidateIdempotencyKey(""); !errors.As(err, &p) || p.Reason != "missing-idempotency-key" {
		t.Fatalf("empty key: %v", err)
	}
	if err := ValidateIdempotencyKey("idem_7f3c9a12-4b0e"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIdempotencyKey("has space"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("space: %v", err)
	}
}
