package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// HoldList is a page of holds with the count per status.
type HoldList struct {
	Data   []dto.Hold    `json:"data"`
	Next   *string       `json:"next,omitempty"`
	Counts dto.HoldStats `json:"counts"`
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
		Parameters:  []*huma.Param{restapi.CursorParam()},
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
	restapi.LedgerPath
	Status  scalar.HoldStatus    `query:"status"`
	Address scalar.AddressPrefix `query:"address" doc:"Only holds from or to an account at or below this prefix."`
	restapi.Page
},
) (*restapi.Body[HoldList], error) {
	views, next, counts, err := h.HoldQueries.List(ctx, application.ListHoldsInput{
		Ledger: string(in.Ledger), Status: string(in.Status), Address: string(in.Address), Cursor: in.Cursor, Limit: in.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := HoldList{Data: make([]dto.Hold, len(views)), Next: restapi.Optional(next), Counts: dto.HoldStatsFrom(counts, nil)}
	for i := range views {
		out.Data[i] = dto.HoldFrom(&views[i])
	}
	return &restapi.Body[HoldList]{Body: out}, nil
}

func (h *handlers) createHold(ctx context.Context, in *struct {
	restapi.LedgerPath
	WriteKey
	Body CreateHoldRequest
},
) (*restapi.Body[dto.Hold], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	b := &in.Body
	posting, err := dto.Posting{Source: b.Source, Destination: b.Destination, Asset: b.Asset, Amount: b.Amount}.Parse()
	if err != nil {
		return nil, err
	}
	timeout := deref(b.TimeoutSeconds)
	if timeout < 0 || timeout > domain.MaxHoldTimeout {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "timeoutSeconds must be between 0 and 4294967295")
	}
	v, err := h.HoldCommands.Create(ctx, application.CreateHoldInput{
		Ledger:         string(in.Ledger),
		Key:            in.IdempotencyKey,
		RequestID:      requestid.From(ctx),
		Posting:        posting,
		TimeoutSeconds: uint32(timeout),
		Reference:      deref(b.Reference),
		EventType:      deref(b.EventType),
		Metadata:       b.Metadata.Map(),
	})
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Hold]{Body: dto.HoldFrom(&v)}, nil
}

func (h *handlers) getHold(ctx context.Context, in *struct {
	restapi.LedgerPath
	ResourcePath
},
) (*restapi.Body[dto.Hold], error) {
	v, err := h.HoldQueries.Get(ctx, string(in.Ledger), string(in.ID))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Hold]{Body: dto.HoldFrom(&v)}, nil
}

func (h *handlers) captureHold(ctx context.Context, in *struct {
	restapi.LedgerPath
	ResourcePath
	WriteKey
	Body *CaptureHoldRequest
},
) (*restapi.Body[dto.Hold], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	capture := application.CaptureHoldInput{Ledger: string(in.Ledger), Key: in.IdempotencyKey, RequestID: requestid.From(ctx), ID: string(in.ID)}
	if in.Body != nil && in.Body.Amount != nil {
		a, err := ledger.ParseAmount(string(*in.Body.Amount))
		if err != nil {
			return nil, err
		}
		capture.Amount = &a
	}
	v, err := h.HoldCommands.Capture(ctx, capture)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Hold]{Body: dto.HoldFrom(&v)}, nil
}

func (h *handlers) voidHold(ctx context.Context, in *struct {
	restapi.LedgerPath
	ResourcePath
	WriteKey
},
) (*restapi.Body[dto.Hold], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	v, err := h.HoldCommands.Void(ctx, application.VoidHoldInput{Ledger: string(in.Ledger), Key: in.IdempotencyKey, RequestID: requestid.From(ctx), ID: string(in.ID)})
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Hold]{Body: dto.HoldFrom(&v)}, nil
}
