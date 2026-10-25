package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
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

func registerHolds(api huma.API, h *handlers) {
	tags := []string{"holds"}
	restapi.Register(api, huma.Operation{
		OperationID: "listHolds", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/holds", Tags: tags,
		Summary:     "List holds",
		Description: "Holds newest first. `expired` is derived from the cluster clock and the hold timeout.",
		Parameters:  []*huma.Param{cursorParam()},
		Responses:   restapi.Success(http.StatusOK, "A page of holds."),
	}, h.listHolds)

	restapi.Register(api, huma.Operation{
		OperationID: "createHold", Method: http.MethodPost, Path: "/v1/ledgers/{ledger}/holds", Tags: tags,
		Summary: "Create a hold",
		Description: "Reserves an amount with a TigerBeetle pending transfer (ADR-0010). The source's available\n" +
			"balance drops by the amount at once; nothing moves until the hold is captured.\n",
		Parameters:    []*huma.Param{idempotencyKeyParam()},
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The hold is open."),
	}, h.createHold)

	restapi.Register(api, huma.Operation{
		OperationID: "getHold", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/holds/{id}", Tags: tags,
		Summary:     "Get a hold",
		Description: "The hold, its pending transfer and, once resolved, the transfer that resolved it.",
		Responses:   restapi.Success(http.StatusOK, "The hold."),
	}, h.getHold)

	restapi.Register(api, huma.Operation{
		OperationID: "captureHold", Method: http.MethodPost, Path: "/v1/ledgers/{ledger}/holds/{id}/capture", Tags: tags,
		Summary: "Capture a hold",
		Description: "Posts the pending transfer. Without an amount the full hold is captured; a smaller amount\n" +
			"captures that much and releases the rest. A hold is captured once (409\n" +
			"`hold-already-resolved`) and not after it expired (409 `hold-expired`).\n",
		Parameters: []*huma.Param{idempotencyKeyParam()},
		Responses:  restapi.Success(http.StatusOK, "The hold is captured."),
	}, h.captureHold)

	restapi.Register(api, huma.Operation{
		OperationID: "voidHold", Method: http.MethodPost, Path: "/v1/ledgers/{ledger}/holds/{id}/void", Tags: tags,
		Summary:     "Void a hold",
		Description: "Voids the pending transfer and releases the whole amount to the source.",
		Parameters:  []*huma.Param{idempotencyKeyParam()},
		Responses:   restapi.Success(http.StatusOK, "The hold is voided."),
	}, h.voidHold)
}

func (h *handlers) listHolds(ctx context.Context, in *struct {
	LedgerPath
	Status  scalar.HoldStatus    `query:"status"`
	Address scalar.AddressPrefix `query:"address" doc:"Only holds from or to an account at or below this prefix."`
	Page
},
) (*body[HoldList], error) {
	views, next, counts, err := h.Holds.List(ctx, string(in.Ledger), string(in.Status), string(in.Address), in.Cursor, in.Limit)
	if err != nil {
		return nil, err
	}
	out := HoldList{Data: make([]Hold, len(views)), Next: optional(next), Counts: holdStatsOf(counts, nil)}
	for i := range views {
		out.Data[i] = holdOf(&views[i])
	}
	return &body[HoldList]{Body: out}, nil
}

func (h *handlers) createHold(ctx context.Context, in *struct {
	LedgerPath
	WriteKey
	Body CreateHoldRequest
},
) (*body[Hold], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	b := &in.Body
	posting, err := postingIn(b.Source, b.Destination, b.Asset, b.Amount)
	if err != nil {
		return nil, err
	}
	timeout := deref(b.TimeoutSeconds)
	if timeout < 0 || timeout > ledger.MaxHoldTimeout {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "timeoutSeconds must be between 0 and 4294967295")
	}
	v, err := h.Holds.Create(ctx, app.CreateHoldInput{
		Ledger:         string(in.Ledger),
		Key:            in.IdempotencyKey,
		RequestID:      requestid.From(ctx),
		Posting:        posting,
		TimeoutSeconds: uint32(timeout),
		Reference:      deref(b.Reference),
		EventType:      deref(b.EventType),
		Metadata:       metadataIn(b.Metadata),
	})
	if err != nil {
		return nil, err
	}
	return &body[Hold]{Body: holdOf(&v)}, nil
}

func (h *handlers) getHold(ctx context.Context, in *struct {
	LedgerPath
	ResourcePath
},
) (*body[Hold], error) {
	v, err := h.Holds.Get(ctx, string(in.Ledger), string(in.ID))
	if err != nil {
		return nil, err
	}
	return &body[Hold]{Body: holdOf(&v)}, nil
}

func (h *handlers) captureHold(ctx context.Context, in *struct {
	LedgerPath
	ResourcePath
	WriteKey
	Body *CaptureHoldRequest
},
) (*body[Hold], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	var amount *ledger.Amount
	if in.Body != nil && in.Body.Amount != nil {
		a, err := ledger.ParseAmount(string(*in.Body.Amount))
		if err != nil {
			return nil, err
		}
		amount = &a
	}
	v, err := h.Holds.Capture(ctx, string(in.Ledger), in.IdempotencyKey, requestid.From(ctx), string(in.ID), amount)
	if err != nil {
		return nil, err
	}
	return &body[Hold]{Body: holdOf(&v)}, nil
}

func (h *handlers) voidHold(ctx context.Context, in *struct {
	LedgerPath
	ResourcePath
	WriteKey
},
) (*body[Hold], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	v, err := h.Holds.Void(ctx, string(in.Ledger), in.IdempotencyKey, requestid.From(ctx), string(in.ID))
	if err != nil {
		return nil, err
	}
	return &body[Hold]{Body: holdOf(&v)}, nil
}

func holdOf(v *app.HoldView) Hold {
	out := Hold{
		ID:                   scalar.ID(v.ID.String()),
		Ledger:               scalar.LedgerName(v.Ledger),
		Source:               scalar.Address(v.Source),
		Destination:          scalar.Address(v.Destination),
		Asset:                scalar.Asset(v.Asset),
		Amount:               scalar.Amount(v.Amount.String()),
		Status:               scalar.HoldStatus(v.Status),
		TimeoutSeconds:       int64(v.TimeoutSeconds),
		CreatedAt:            v.CreatedAt,
		Timestamp:            timestampOf(v.Timestamp),
		ExpiresAt:            v.ExpiresAt,
		ResolvedAt:           v.ResolvedAt,
		Reference:            optional(v.Reference),
		EventType:            eventTypeRefOf(v.EventType),
		Metadata:             metadataOf(v.Metadata),
		IdempotencyKey:       v.IdempotencyKey,
		PendingTransferID:    scalar.ID(v.PendingTransferID.String()),
		ResolutionTransferID: optionalID(v.ResolutionTransferID),
		TbLedger:             int64(v.TBLedger),
	}
	if v.CapturedAmount != nil {
		c := scalar.Amount(v.CapturedAmount.String())
		out.CapturedAmount = &c
	}
	return out
}
