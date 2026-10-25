package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

// OverviewServer implements the LedgerService methods bookkeeping owns: assets, stats and search.
// internal/transport/grpc joins it with the ledgers module's part.
type OverviewServer struct {
	accounts application.AccountQueries
	overview application.OverviewQueries
}

// NewOverviewServer builds the overview methods.
func NewOverviewServer(app application.Application) *OverviewServer {
	return &OverviewServer{accounts: app.AccountQueries, overview: app.OverviewQueries}
}

// ListAssets lists the assets of a ledger.
func (s *OverviewServer) ListAssets(ctx context.Context, req *pb.ListAssetsRequest) (*pb.ListAssetsResponse, error) {
	assets, err := s.accounts.Assets(ctx, req.GetLedger())
	if err != nil {
		return nil, err
	}
	return &pb.ListAssetsResponse{Assets: assetSummariesOf(dto.AssetSummariesFrom(assets))}, nil
}

// GetLedgerStats reads the counts and amounts of the ledger overview.
func (s *OverviewServer) GetLedgerStats(ctx context.Context, req *pb.GetLedgerStatsRequest) (*pb.GetLedgerStatsResponse, error) {
	st, err := s.overview.Stats(ctx, req.GetLedger())
	if err != nil {
		return nil, err
	}
	stats := dto.LedgerStatsFrom(&st)
	reasons := make([]*pb.DeclinedReason, len(stats.Declined.Reasons))
	for i, r := range stats.Declined.Reasons {
		reasons[i] = &pb.DeclinedReason{Reason: r.Reason, Count: r.Count}
	}
	return &pb.GetLedgerStatsResponse{Stats: &pb.LedgerStats{
		Accounts:      stats.Accounts,
		AccountsToday: stats.AccountsToday,
		Assets:        assetSummariesOf(stats.Assets),
		Holds:         holdStatsOf(&stats.Holds),
		Declined:      &pb.DeclinedStats{Total: stats.Declined.Total, Reasons: reasons},
	}}, nil
}

// Search finds what a query names.
func (s *OverviewServer) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	r, err := s.overview.Search(ctx, req.GetLedger(), req.GetQuery())
	if err != nil {
		return nil, err
	}
	res := dto.SearchResultsFrom(req.GetQuery(), &r)
	out := &pb.SearchResponse{
		Query:        res.Query,
		Accounts:     make([]*pb.SearchAccount, len(res.Accounts)),
		Transactions: make([]*pb.SearchTransaction, len(res.Transactions)),
		Holds:        make([]*pb.SearchHold, len(res.Holds)),
		Requests:     make([]*pb.SearchWriteRequest, len(res.Requests)),
	}
	for i, a := range res.Accounts {
		out.Accounts[i] = &pb.SearchAccount{Address: string(a.Address), Assets: assetsOf(a.Assets)}
	}
	for i, t := range res.Transactions {
		out.Transactions[i] = &pb.SearchTransaction{Id: string(t.ID), Reference: t.Reference, CommittedAt: grpcapi.Timestamp(t.CommittedAt)}
	}
	for i, h := range res.Holds {
		out.Holds[i] = &pb.SearchHold{Id: string(h.ID), Status: grpcapi.EnumOf[pb.HoldStatus](string(h.Status))}
	}
	for i, q := range res.Requests {
		out.Requests[i] = &pb.SearchWriteRequest{
			Key: q.Key, Operation: grpcapi.EnumOf[pb.Operation](string(q.Operation)), Status: grpcapi.EnumOf[pb.RequestStatus](string(q.Status)),
		}
	}
	return out, nil
}

func assetSummariesOf(in []dto.AssetSummary) []*pb.AssetSummary {
	out := make([]*pb.AssetSummary, len(in))
	for i, a := range in {
		out[i] = &pb.AssetSummary{
			Asset: string(a.Asset), TbLedger: a.TbLedger, CreatedAt: grpcapi.Timestamp(a.CreatedAt),
			Accounts: a.Accounts, Issued: string(a.Issued),
		}
	}
	return out
}
