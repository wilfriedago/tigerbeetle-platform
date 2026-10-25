package domain

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// The change event types TigerBeetle CDC reports, one per committed transfer (ADR-0022).
const (
	ChangeSinglePhase     = "single_phase"
	ChangeTwoPhasePending = "two_phase_pending"
	ChangeTwoPhasePosted  = "two_phase_posted"
	ChangeTwoPhaseVoided  = "two_phase_voided"
	ChangeTwoPhaseExpired = "two_phase_expired"
)

// ChangeEvent is one committed TigerBeetle transfer as the CDC job reports it: the transfer and
// both of its accounts, with their balances as of this event. Every leg of a linked chain is its
// own event. TigerBeetle timestamps are unique, so Timestamp identifies the event.
type ChangeEvent struct {
	Type      string
	Timestamp uint64
	TBLedger  uint32
	Transfer  ChangeTransfer
	Debit     ChangeAccount
	Credit    ChangeAccount
}

// ChangeTransfer is the transfer of a change event.
type ChangeTransfer struct {
	ID        ledger.ID
	Amount    ledger.Amount
	PendingID ledger.ID
	// UserData128 is the API transaction or hold ID for transfers this platform wrote.
	UserData128 ledger.ID
	UserData64  uint64
	UserData32  uint32
	Timeout     uint32
	Code        uint16
	Flags       uint16
	Timestamp   uint64
}

// ChangeAccount is an account of a change event, with its balances as of the event.
type ChangeAccount struct {
	ID             ledger.ID
	DebitsPending  ledger.Amount
	DebitsPosted   ledger.Amount
	CreditsPending ledger.Amount
	CreditsPosted  ledger.Amount
	UserData128    ledger.ID
	UserData64     uint64
	UserData32     uint32
	Code           uint16
	Flags          uint16
	Timestamp      uint64
}

// LedgerEvent is a change event with what Postgres knew before the transfer existed: the ledger
// and asset of its TigerBeetle ledger, and the address of each account. A transfer another
// TigerBeetle client wrote has none of them, and they stay empty. Transaction metadata and
// references are written after the commit and could race, so they are left out.
type LedgerEvent struct {
	ChangeEvent
	Ledger        string
	Asset         string
	DebitAddress  string
	CreditAddress string
}

// LedgerEventPublisher sends ledger events to their stream, in order. Publish returns once the
// stream has acknowledged every event, so the caller may acknowledge its source.
type LedgerEventPublisher interface {
	Publish(ctx context.Context, events ...LedgerEvent) error
}
