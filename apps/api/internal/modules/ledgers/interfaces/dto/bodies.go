package dto

import (
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Request bodies and list answers: the same JSON on REST and the broker RPC transports (plan
// section 11.2), named as the REST spec names them.

// LedgerList is a page of ledgers.
type LedgerList struct {
	Data []Ledger `json:"data"`
	Next *string  `json:"next,omitempty" doc:"The cursor of the next page; absent on the last page."`
}

// CreateLedgerRequest is the body of POST /v1/ledgers/{ledger}.
type CreateLedgerRequest struct {
	Metadata scalar.Metadata `json:"metadata,omitempty"`
}

// RuleList lists the rules of a ledger, most specific first.
type RuleList struct {
	Data []Rule `json:"data"`
}

// PutRuleRequest is the body of PUT .../rules/{pattern}.
type PutRuleRequest struct {
	Limit   scalar.BalanceLimit `json:"limit"`
	History *bool               `json:"history,omitempty" default:"false"`
	Code    int                 `json:"code" minimum:"1" maximum:"65535"`
}

// EventTypeList lists the event types of a ledger.
type EventTypeList struct {
	Data []EventType `json:"data"`
}

// CreateEventTypeRequest is the body of POST .../event-types.
type CreateEventTypeRequest struct {
	Name string `json:"name" maxLength:"63"`
	Code *int   `json:"code,omitempty" minimum:"2" maximum:"65535"`
}
