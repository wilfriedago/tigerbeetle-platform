// Package v1 is the bookkeeping module's gRPC interface (ADR-0021): AccountService,
// TransactionService, HoldService, WriteRequestService, and its parts of LedgerService (assets,
// stats, search) and OperationsService (the reconciler). It calls the same use cases as REST and
// GraphQL, so a write retried on any transport with the same idempotency key replays. Errors are
// returned as Go errors; internal/platform/grpcserver maps them to statuses.
package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// AccountServer implements AccountService.
type AccountServer struct {
	commands application.AccountCommands
	queries  application.AccountQueries
}

// NewAccountServer builds AccountService.
func NewAccountServer(app application.Application) *AccountServer {
	return &AccountServer{commands: app.AccountCommands, queries: app.AccountQueries}
}

// ListAccounts lists accounts in address order.
func (s *AccountServer) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.ListAccountsResponse, error) {
	views, next, err := s.queries.List(ctx, application.ListAccountsInput{
		Ledger: req.GetLedger(), Prefix: req.GetPrefix(), Search: req.GetSearch(), Asset: req.GetAsset(),
		Cursor: req.GetPageToken(), Limit: grpcapi.PageSize(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}
	out := &pb.ListAccountsResponse{Accounts: make([]*pb.AccountSummary, len(views)), NextPageToken: next}
	for i := range views {
		summary := dto.AccountSummaryFrom(&views[i])
		out.Accounts[i] = accountSummaryOf(&summary)
	}
	return out, nil
}

// GetAccount reads an account by address.
func (s *AccountServer) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	v, err := s.queries.Get(ctx, req.GetLedger(), req.GetAddress())
	if err != nil {
		return nil, err
	}
	account := dto.AccountFrom(&v)
	return &pb.GetAccountResponse{Account: accountOf(&account)}, nil
}

// GetAccountHistory reads point-in-time balances of one account, newest first.
func (s *AccountServer) GetAccountHistory(ctx context.Context, req *pb.GetAccountHistoryRequest) (*pb.GetAccountHistoryResponse, error) {
	points, err := s.queries.History(ctx, req.GetLedger(), req.GetAddress(), req.GetAsset(), int(req.GetLimit()))
	if err != nil {
		return nil, err
	}
	h := dto.BalanceHistoryFrom(scalar.Address(req.GetAddress()), scalar.Asset(req.GetAsset()), points)
	out := &pb.GetAccountHistoryResponse{Address: string(h.Address), Asset: string(h.Asset), Points: make([]*pb.BalancePoint, len(h.Data))}
	for i, p := range h.Data {
		out.Points[i] = &pb.BalancePoint{
			Timestamp: string(p.Timestamp), At: grpcapi.Timestamp(p.At), Balance: string(p.Balance),
			Reserved: string(p.Reserved), Credits: string(p.Credits), Debits: string(p.Debits),
		}
	}
	return out, nil
}

// ReplaceAccountMetadata replaces the metadata of an account.
func (s *AccountServer) ReplaceAccountMetadata(ctx context.Context, req *pb.ReplaceAccountMetadataRequest) (*pb.ReplaceAccountMetadataResponse, error) {
	v, err := s.commands.ReplaceMetadata(ctx, req.GetLedger(), req.GetAddress(), metadataIn(req.GetMetadata()))
	if err != nil {
		return nil, err
	}
	account := dto.AccountFrom(&v)
	return &pb.ReplaceAccountMetadataResponse{Account: accountOf(&account)}, nil
}

// AggregateBalances sums the balances of every account under a prefix, per asset.
func (s *AccountServer) AggregateBalances(ctx context.Context, req *pb.AggregateBalancesRequest) (*pb.AggregateBalancesResponse, error) {
	agg, err := s.queries.AggregateBalances(ctx, req.GetLedger(), req.GetAddress())
	if err != nil {
		return nil, err
	}
	a := dto.AggregateBalancesFrom(&agg)
	out := &pb.AggregateBalancesResponse{
		Prefix: a.Prefix, AsOf: grpcapi.Timestamp(a.AsOf), Accounts: a.Accounts, Chunks: int32(a.Chunks),
		Assets: make([]*pb.AggregateBalance, len(a.Assets)),
	}
	for i, b := range a.Assets {
		out.Assets[i] = &pb.AggregateBalance{
			Asset: string(b.Asset), Accounts: b.Accounts, Balance: string(b.Balance),
			Available: string(b.Available), Reserved: string(b.Reserved), Incoming: string(b.Incoming),
		}
	}
	return out, nil
}

// ListSegments lists the next address segment under a prefix.
func (s *AccountServer) ListSegments(ctx context.Context, req *pb.ListSegmentsRequest) (*pb.ListSegmentsResponse, error) {
	prefix, rows, err := s.queries.Segments(ctx, req.GetLedger(), req.GetPrefix())
	if err != nil {
		return nil, err
	}
	out := &pb.ListSegmentsResponse{Prefix: grpcapi.Optional(prefix), Segments: make([]*pb.Segment, len(rows))}
	for i := range rows {
		seg := dto.SegmentFrom(&rows[i])
		out.Segments[i] = &pb.Segment{
			Segment: seg.Segment, Prefix: seg.Prefix, Accounts: seg.Accounts, Assets: assetsOf(seg.Assets),
			IsAccount: seg.IsAccount, HasChildren: seg.HasChildren,
		}
	}
	return out, nil
}

// metadataIn turns an absent map into an empty one: replacing metadata with nothing clears it.
func metadataIn(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
