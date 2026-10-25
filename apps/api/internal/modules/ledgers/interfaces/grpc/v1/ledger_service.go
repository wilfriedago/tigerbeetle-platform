// Package v1 is the ledgers module's gRPC interface (ADR-0021): its part of LedgerService and the
// whole SettingsService, calling the same use cases as REST and GraphQL. Errors are returned as
// Go errors; internal/platform/grpcserver maps them to statuses.
package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

// LedgerServer implements the LedgerService methods this module owns. The rest of the service
// (assets, stats, search) belongs to bookkeeping; internal/transport/grpc joins the two.
type LedgerServer struct {
	commands application.LedgerCommands
	queries  application.LedgerQueries
}

// NewLedgerServer builds the ledger methods.
func NewLedgerServer(app application.Application) *LedgerServer {
	return &LedgerServer{commands: app.LedgerCommands, queries: app.LedgerQueries}
}

// ListLedgers lists every ledger in name order.
func (s *LedgerServer) ListLedgers(ctx context.Context, req *pb.ListLedgersRequest) (*pb.ListLedgersResponse, error) {
	views, next, err := s.queries.List(ctx, req.GetPageToken(), grpcapi.PageSize(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	out := &pb.ListLedgersResponse{Ledgers: make([]*pb.Ledger, len(views)), NextPageToken: next}
	for i := range views {
		out.Ledgers[i] = ledgerOf(dto.LedgerFrom(views[i]))
	}
	return out, nil
}

// GetLedger reads a ledger by name.
func (s *LedgerServer) GetLedger(ctx context.Context, req *pb.GetLedgerRequest) (*pb.GetLedgerResponse, error) {
	v, err := s.queries.Get(ctx, req.GetLedger())
	if err != nil {
		return nil, err
	}
	return &pb.GetLedgerResponse{Ledger: ledgerOf(dto.LedgerFrom(v))}, nil
}

// CreateLedger creates a ledger.
func (s *LedgerServer) CreateLedger(ctx context.Context, req *pb.CreateLedgerRequest) (*pb.CreateLedgerResponse, error) {
	v, err := s.commands.Create(ctx, req.GetLedger(), req.GetMetadata())
	if err != nil {
		return nil, err
	}
	return &pb.CreateLedgerResponse{Ledger: ledgerOf(dto.LedgerFrom(v))}, nil
}

// ReplaceLedgerMetadata replaces the metadata of a ledger.
func (s *LedgerServer) ReplaceLedgerMetadata(ctx context.Context, req *pb.ReplaceLedgerMetadataRequest) (*pb.ReplaceLedgerMetadataResponse, error) {
	v, err := s.commands.ReplaceMetadata(ctx, req.GetLedger(), metadataIn(req.GetMetadata()))
	if err != nil {
		return nil, err
	}
	return &pb.ReplaceLedgerMetadataResponse{Ledger: ledgerOf(dto.LedgerFrom(v))}, nil
}

func ledgerOf(l dto.Ledger) *pb.Ledger {
	assets := make([]string, len(l.Assets))
	for i, a := range l.Assets {
		assets[i] = string(a)
	}
	return &pb.Ledger{
		Name:      string(l.Name),
		Metadata:  l.Metadata.Map(),
		CreatedAt: grpcapi.Timestamp(l.CreatedAt),
		Assets:    assets,
		Accounts:  l.Accounts,
	}
}

// metadataIn turns an absent map into an empty one: replacing metadata with nothing clears it, as
// a PUT of {} does on REST.
func metadataIn(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
