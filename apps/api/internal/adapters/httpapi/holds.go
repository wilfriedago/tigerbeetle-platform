package httpapi

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

func itoa(i int) string {
	return strconv.Itoa(i)
}

// jsonObject decodes a stored JSON object for display; anything else becomes an empty object.
func jsonObject(raw []byte) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// ListHolds returns a page of holds with the count per status.
func (s *server) ListHolds(ctx context.Context, req gen.ListHoldsRequestObject) (gen.ListHoldsResponseObject, error) {
	p := req.Params
	views, next, counts, err := s.holds.List(ctx, req.Ledger, string(deref(p.Status)), deref(p.Address), deref(p.Cursor), deref(p.Limit))
	if err != nil {
		return nil, err
	}
	data := make([]gen.Hold, len(views))
	for i := range views {
		v := &views[i]
		data[i] = holdOf(*v)
	}
	return gen.ListHolds200JSONResponse{Data: data, Next: optional(next), Counts: holdStatsOf(counts, nil)}, nil
}

// CreateHold reserves an amount with a pending transfer.
func (s *server) CreateHold(ctx context.Context, req gen.CreateHoldRequestObject) (gen.CreateHoldResponseObject, error) {
	if req.Body == nil {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "a JSON body is required")
	}
	b := req.Body
	posting, err := postingIn(b.Source, b.Destination, b.Asset, b.Amount)
	if err != nil {
		return nil, err
	}
	timeout := deref(b.TimeoutSeconds)
	if timeout < 0 || timeout > ledger.MaxHoldTimeout {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "timeoutSeconds must be between 0 and 4294967295")
	}
	v, err := s.holds.Create(ctx, app.CreateHoldInput{
		Ledger:         req.Ledger,
		Key:            req.Params.IdempotencyKey,
		RequestID:      RequestIDFrom(ctx),
		Posting:        posting,
		TimeoutSeconds: uint32(timeout),
		Reference:      deref(b.Reference),
		EventType:      deref(b.EventType),
		Metadata:       metadataIn(b.Metadata),
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateHold201JSONResponse(holdOf(v)), nil
}

// GetHold returns a hold.
func (s *server) GetHold(ctx context.Context, req gen.GetHoldRequestObject) (gen.GetHoldResponseObject, error) {
	v, err := s.holds.Get(ctx, req.Ledger, req.Id)
	if err != nil {
		return nil, err
	}
	return gen.GetHold200JSONResponse(holdOf(v)), nil
}

// CaptureHold posts the pending transfer, fully or partially.
func (s *server) CaptureHold(ctx context.Context, req gen.CaptureHoldRequestObject) (gen.CaptureHoldResponseObject, error) {
	var amount *ledger.Amount
	if req.Body != nil && req.Body.Amount != nil {
		a, err := ledger.ParseAmount(*req.Body.Amount)
		if err != nil {
			return nil, err
		}
		amount = &a
	}
	v, err := s.holds.Capture(ctx, req.Ledger, req.Params.IdempotencyKey, RequestIDFrom(ctx), req.Id, amount)
	if err != nil {
		return nil, err
	}
	return gen.CaptureHold200JSONResponse(holdOf(v)), nil
}

// VoidHold releases the hold.
func (s *server) VoidHold(ctx context.Context, req gen.VoidHoldRequestObject) (gen.VoidHoldResponseObject, error) {
	v, err := s.holds.Void(ctx, req.Ledger, req.Params.IdempotencyKey, RequestIDFrom(ctx), req.Id)
	if err != nil {
		return nil, err
	}
	return gen.VoidHold200JSONResponse(holdOf(v)), nil
}
