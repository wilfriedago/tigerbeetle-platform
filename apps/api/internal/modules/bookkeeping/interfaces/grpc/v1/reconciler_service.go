package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

// ReconcilerServer implements OperationsService.GetReconcilerStatus. internal/transport/grpc joins
// it with the health module's GetInfo.
type ReconcilerServer struct {
	reconciler application.ReconcilerQueries
}

// NewReconcilerServer builds the reconciler method.
func NewReconcilerServer(app application.Application) *ReconcilerServer {
	return &ReconcilerServer{reconciler: app.Reconciler}
}

// GetReconcilerStatus reads the state of the reconciler.
func (s *ReconcilerServer) GetReconcilerStatus(ctx context.Context, _ *pb.GetReconcilerStatusRequest) (*pb.GetReconcilerStatusResponse, error) {
	st, err := s.reconciler.Status(ctx)
	if err != nil {
		return nil, err
	}
	r := dto.ReconcilerStatusFrom(&st)
	out := &pb.GetReconcilerStatusResponse{
		Running:            r.Running,
		GracePeriodSeconds: int32(r.GracePeriodSeconds),
		IntervalSeconds:    int32(r.IntervalSeconds),
		LastRunAt:          grpcapi.OptionalTimestamp(r.LastRunAt),
		Pending:            r.Pending,
		Stuck:              r.Stuck,
		StuckRequests:      make([]*pb.StuckRequest, len(r.StuckRequests)),
	}
	if r.LastSettled != nil {
		settled := int32(*r.LastSettled)
		out.LastSettled = &settled
	}
	for i, q := range r.StuckRequests {
		out.StuckRequests[i] = &pb.StuckRequest{
			Ledger: string(q.Ledger), Key: q.Key, Operation: grpcapi.EnumOf[pb.Operation](string(q.Operation)),
			CreatedAt: grpcapi.Timestamp(q.CreatedAt), Attempts: int32(q.Attempts), Note: q.Note,
		}
	}
	return out, nil
}
