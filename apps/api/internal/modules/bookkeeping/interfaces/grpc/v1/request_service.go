package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

// WriteRequestServer implements WriteRequestService.
type WriteRequestServer struct {
	queries application.RequestQueries
}

// NewWriteRequestServer builds WriteRequestService.
func NewWriteRequestServer(app application.Application) *WriteRequestServer {
	return &WriteRequestServer{queries: app.RequestQueries}
}

// ListWriteRequests lists write requests, newest first.
func (s *WriteRequestServer) ListWriteRequests(ctx context.Context, req *pb.ListWriteRequestsRequest) (*pb.ListWriteRequestsResponse, error) {
	rows, next, err := s.queries.List(ctx, application.ListRequestsInput{
		Ledger: req.GetLedger(), Status: grpcapi.ValueOf(req.GetStatus()), Operation: grpcapi.ValueOf(req.GetOperation()),
		Search: req.GetSearch(), Cursor: req.GetPageToken(), Limit: grpcapi.PageSize(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}
	out := &pb.ListWriteRequestsResponse{Requests: make([]*pb.WriteRequest, len(rows)), NextPageToken: next}
	for i := range rows {
		r := dto.WriteRequestFrom(&rows[i])
		out.Requests[i] = writeRequestOf(&r)
	}
	return out, nil
}

// GetWriteRequest reads a write request by Idempotency-Key. The stored bodies go out as the JSON
// text Postgres holds, so no number passes through a double.
func (s *WriteRequestServer) GetWriteRequest(ctx context.Context, req *pb.GetWriteRequestRequest) (*pb.GetWriteRequestResponse, error) {
	rec, err := s.queries.Get(ctx, req.GetLedger(), req.GetKey())
	if err != nil {
		return nil, err
	}
	detail := dto.WriteRequestDetailFrom(&rec)
	out := &pb.GetWriteRequestResponse{
		Request:     writeRequestOf(&detail.WriteRequest),
		RequestJson: string(rec.Request),
		TransferIds: make([]string, len(detail.TransferIDs)),
	}
	if len(rec.Response) > 0 {
		out.ResponseJson = grpcapi.Optional(string(rec.Response))
	}
	for i, id := range detail.TransferIDs {
		out.TransferIds[i] = string(id)
	}
	return out, nil
}
