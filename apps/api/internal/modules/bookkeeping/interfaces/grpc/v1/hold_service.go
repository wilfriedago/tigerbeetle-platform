package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// HoldServer implements HoldService.
type HoldServer struct {
	commands application.HoldCommands
	queries  application.HoldQueries
}

// NewHoldServer builds HoldService.
func NewHoldServer(app application.Application) *HoldServer {
	return &HoldServer{commands: app.HoldCommands, queries: app.HoldQueries}
}

// ListHolds lists holds, newest first, with the counts by status.
func (s *HoldServer) ListHolds(ctx context.Context, req *pb.ListHoldsRequest) (*pb.ListHoldsResponse, error) {
	views, next, counts, err := s.queries.List(ctx, application.ListHoldsInput{
		Ledger: req.GetLedger(), Status: grpcapi.ValueOf(req.GetStatus()), Address: req.GetAddress(),
		Cursor: req.GetPageToken(), Limit: grpcapi.PageSize(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}
	stats := dto.HoldStatsFrom(counts, nil)
	out := &pb.ListHoldsResponse{Holds: make([]*pb.Hold, len(views)), NextPageToken: next, Counts: holdStatsOf(&stats)}
	for i := range views {
		h := dto.HoldFrom(&views[i])
		out.Holds[i] = holdOf(&h)
	}
	return out, nil
}

// GetHold reads a hold by ID.
func (s *HoldServer) GetHold(ctx context.Context, req *pb.GetHoldRequest) (*pb.GetHoldResponse, error) {
	v, err := s.queries.Get(ctx, req.GetLedger(), req.GetId())
	if err != nil {
		return nil, err
	}
	h := dto.HoldFrom(&v)
	return &pb.GetHoldResponse{Hold: holdOf(&h)}, nil
}

// CreateHold reserves an amount on the source.
func (s *HoldServer) CreateHold(ctx context.Context, req *pb.CreateHoldRequest) (*pb.CreateHoldResponse, error) {
	posting, err := dto.Posting{
		Source: scalar.Address(req.GetSource()), Destination: scalar.Address(req.GetDestination()),
		Asset: scalar.Asset(req.GetAsset()), Amount: scalar.Amount(req.GetAmount()),
	}.Parse()
	if err != nil {
		return nil, err
	}
	timeout := req.GetTimeoutSeconds()
	if timeout < 0 || timeout > domain.MaxHoldTimeout {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "timeout_seconds must be between 0 and 4294967295")
	}
	v, err := s.commands.Create(ctx, application.CreateHoldInput{
		Ledger:         req.GetLedger(),
		Key:            req.GetIdempotencyKey(),
		RequestID:      requestid.From(ctx),
		Posting:        posting,
		TimeoutSeconds: uint32(timeout),
		Reference:      req.GetReference(),
		EventType:      req.GetEventType(),
		Metadata:       metadataIn(req.GetMetadata()),
	})
	if err != nil {
		return nil, err
	}
	h := dto.HoldFrom(&v)
	return &pb.CreateHoldResponse{Hold: holdOf(&h)}, nil
}

// CaptureHold posts all or part of a hold to the destination.
func (s *HoldServer) CaptureHold(ctx context.Context, req *pb.CaptureHoldRequest) (*pb.CaptureHoldResponse, error) {
	capture := application.CaptureHoldInput{Ledger: req.GetLedger(), Key: req.GetIdempotencyKey(), RequestID: requestid.From(ctx), ID: req.GetId()}
	if req.Amount != nil {
		a, err := ledger.ParseAmount(req.GetAmount())
		if err != nil {
			return nil, err
		}
		capture.Amount = &a
	}
	v, err := s.commands.Capture(ctx, capture)
	if err != nil {
		return nil, err
	}
	h := dto.HoldFrom(&v)
	return &pb.CaptureHoldResponse{Hold: holdOf(&h)}, nil
}

// VoidHold releases a hold to the source.
func (s *HoldServer) VoidHold(ctx context.Context, req *pb.VoidHoldRequest) (*pb.VoidHoldResponse, error) {
	v, err := s.commands.Void(ctx, application.VoidHoldInput{
		Ledger: req.GetLedger(), Key: req.GetIdempotencyKey(), RequestID: requestid.From(ctx), ID: req.GetId(),
	})
	if err != nil {
		return nil, err
	}
	h := dto.HoldFrom(&v)
	return &pb.VoidHoldResponse{Hold: holdOf(&h)}, nil
}
