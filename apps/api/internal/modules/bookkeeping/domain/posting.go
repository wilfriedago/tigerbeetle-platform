package domain

import (
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// MaxPostings caps the postings of one transaction. A transaction is one linked chain, which must
// fit in a single TigerBeetle request (ADR-0007); 64 keeps every chain far below the 8189 limit.
const MaxPostings = 64

// MaxReferenceLength bounds the client reference of a transaction or hold.
const MaxReferenceLength = 255

// Posting moves an amount of one asset from a source to a destination. The source is the debit
// account and the destination the credit account, so a user balance is credits minus debits
// (ADR-0007).
type Posting struct {
	Source      ledger.Address
	Destination ledger.Address
	Asset       ledger.Asset
	Amount      ledger.Amount
}

// Validate checks one posting. index is zero-based and only used in messages.
func (p Posting) Validate(index int) error {
	if p.Source == p.Destination {
		return &ledger.Problem{
			Kind:    ledger.ErrInvalid,
			Reason:  "invalid-request",
			Detail:  fmt.Sprintf("posting %d: source and destination are both %s; TigerBeetle requires two different accounts", index+1, p.Source),
			Posting: index,
		}
	}
	if p.Amount.IsZero() {
		return &ledger.Problem{Kind: ledger.ErrInvalid, Reason: "invalid-request", Detail: fmt.Sprintf("posting %d: amount must be greater than zero", index+1), Posting: index}
	}
	return nil
}

// ValidatePostings checks the posting count and each posting.
func ValidatePostings(postings []Posting) error {
	if len(postings) == 0 {
		return fmt.Errorf("%w: a transaction needs at least one posting", ledger.ErrInvalid)
	}
	if len(postings) > MaxPostings {
		return fmt.Errorf("%w: a transaction has at most %d postings, got %d", ledger.ErrInvalid, MaxPostings, len(postings))
	}
	for i, p := range postings {
		if err := p.Validate(i); err != nil {
			return err
		}
	}
	return nil
}

// ValidateReference checks an optional client reference.
func ValidateReference(reference string) error {
	if len(reference) > MaxReferenceLength {
		return fmt.Errorf("%w: reference is longer than %d characters", ledger.ErrInvalid, MaxReferenceLength)
	}
	return nil
}
