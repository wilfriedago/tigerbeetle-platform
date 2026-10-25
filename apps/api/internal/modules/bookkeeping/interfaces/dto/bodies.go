package dto

import (
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Request bodies and list answers: the same JSON on REST and the broker RPC transports (plan
// section 11.2), named as the REST spec names them.

// AccountList is a page of accounts.
type AccountList struct {
	Data []AccountSummary `json:"data"`
	Next *string          `json:"next,omitempty"`
}

// SegmentList lists the child segments of a prefix.
type SegmentList struct {
	Prefix *string   `json:"prefix,omitempty"`
	Data   []Segment `json:"data"`
}

// HoldList is a page of holds with the count per status.
type HoldList struct {
	Data   []Hold    `json:"data"`
	Next   *string   `json:"next,omitempty"`
	Counts HoldStats `json:"counts"`
}

// CreateHoldRequest is the body of POST .../holds.
type CreateHoldRequest struct {
	Source         scalar.Address  `json:"source"`
	Destination    scalar.Address  `json:"destination"`
	Asset          scalar.Asset    `json:"asset"`
	Amount         scalar.Amount   `json:"amount"`
	TimeoutSeconds *int64          `json:"timeoutSeconds,omitempty" minimum:"0" maximum:"4294967295" doc:"Releases the hold after this many seconds on the cluster clock; 0 or absent means never."`
	Reference      *string         `json:"reference,omitempty" maxLength:"255"`
	EventType      *string         `json:"eventType,omitempty" maxLength:"63"`
	Metadata       scalar.Metadata `json:"metadata,omitempty"`
}

// CaptureHoldRequest is the optional body of POST .../capture.
type CaptureHoldRequest struct {
	Amount *scalar.Amount `json:"amount,omitempty" doc:"The amount to capture; the full hold when absent. Must not be zero."`
}

// AssetList lists the assets of a ledger.
type AssetList struct {
	Data []AssetSummary `json:"data"`
}

// TransactionList is a page of transactions.
type TransactionList struct {
	Data []Transaction `json:"data"`
	Next *string       `json:"next,omitempty"`
}

// CreateTransactionRequest is the body of POST .../transactions.
type CreateTransactionRequest struct {
	Postings     []Posting       `json:"postings" minItems:"1" maxItems:"64"`
	Reference    *string         `json:"reference,omitempty" maxLength:"255"`
	EventType    *string         `json:"eventType,omitempty" maxLength:"63" doc:"An event type name of the ledger; the default event type when absent."`
	BusinessTime *time.Time      `json:"businessTime,omitempty"`
	Metadata     scalar.Metadata `json:"metadata,omitempty"`
}

// RevertTransactionRequest is the optional body of POST .../revert.
type RevertTransactionRequest struct {
	Reference *string         `json:"reference,omitempty" maxLength:"255"`
	Metadata  scalar.Metadata `json:"metadata,omitempty"`
}

// WriteRequestList is a page of write requests.
type WriteRequestList struct {
	Data []WriteRequest `json:"data"`
	Next *string        `json:"next,omitempty"`
}
