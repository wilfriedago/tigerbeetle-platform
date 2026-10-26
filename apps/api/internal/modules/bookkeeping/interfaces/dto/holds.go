package dto

import (
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Hold is a TigerBeetle pending transfer and, once resolved, the transfer that resolved it.
type Hold struct {
	ID                   scalar.ID         `json:"id"`
	Ledger               scalar.LedgerName `json:"ledger"`
	Source               scalar.Address    `json:"source"`
	Destination          scalar.Address    `json:"destination"`
	Asset                scalar.Asset      `json:"asset"`
	Amount               scalar.Amount     `json:"amount"`
	CapturedAmount       *scalar.Amount    `json:"capturedAmount,omitempty"`
	Status               scalar.HoldStatus `json:"status"`
	TimeoutSeconds       int64             `json:"timeoutSeconds" doc:"Seconds on the cluster clock; 0 means no timeout."`
	CreatedAt            time.Time         `json:"createdAt"`
	Timestamp            scalar.Timestamp  `json:"timestamp"`
	ExpiresAt            *time.Time        `json:"expiresAt,omitempty"`
	ResolvedAt           *time.Time        `json:"resolvedAt,omitempty"`
	Reference            *string           `json:"reference,omitempty"`
	EventType            EventTypeRef      `json:"eventType"`
	Metadata             scalar.Metadata   `json:"metadata"`
	IdempotencyKey       string            `json:"idempotencyKey"`
	PendingTransferID    scalar.ID         `json:"pendingTransferId"`
	ResolutionTransferID *scalar.ID        `json:"resolutionTransferId,omitempty"`
	// TbLedger is optional in the spec but always written.
	TbLedger int64 `json:"tbLedger" required:"false"`
}

// HoldFrom renders a hold.
func HoldFrom(v *application.HoldView) Hold {
	out := Hold{
		ID:                   scalar.IDOf(v.ID),
		Ledger:               scalar.LedgerName(v.Ledger),
		Source:               scalar.Address(v.Source),
		Destination:          scalar.Address(v.Destination),
		Asset:                scalar.Asset(v.Asset),
		Amount:               scalar.AmountOf(v.Amount),
		Status:               scalar.HoldStatus(v.Status),
		TimeoutSeconds:       int64(v.TimeoutSeconds),
		CreatedAt:            v.CreatedAt,
		Timestamp:            scalar.TimestampOf(v.Timestamp),
		ExpiresAt:            v.ExpiresAt,
		ResolvedAt:           v.ResolvedAt,
		Reference:            restapi.Optional(v.Reference),
		EventType:            EventTypeRefOf(v.EventType),
		Metadata:             scalar.MetadataOf(v.Metadata),
		IdempotencyKey:       v.IdempotencyKey,
		PendingTransferID:    scalar.IDOf(v.PendingTransferID),
		ResolutionTransferID: scalar.OptionalID(v.ResolutionTransferID),
		TbLedger:             int64(v.TBLedger),
	}

	if v.CapturedAmount != nil {
		c := scalar.AmountOf(*v.CapturedAmount)
		out.CapturedAmount = &c
	}

	return out
}
