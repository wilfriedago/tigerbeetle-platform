package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// TransactionServer implements TransactionService.
type TransactionServer struct {
	commands application.TransactionCommands
	queries  application.TransactionQueries
}

// NewTransactionServer builds TransactionService.
func NewTransactionServer(app application.Application) *TransactionServer {
	return &TransactionServer{commands: app.TransactionCommands, queries: app.TransactionQueries}
}

// ListTransactions lists committed transactions, newest first.
func (s *TransactionServer) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	views, next, err := s.queries.List(ctx, application.ListTransactionsInput{
		Ledger:    req.GetLedger(),
		Address:   req.GetAddress(),
		Asset:     req.GetAsset(),
		Reference: req.GetReference(),
		EventType: req.GetEventType(),
		After:     grpcapi.TimeOf(req.GetCommittedAfter()),
		Before:    grpcapi.TimeOf(req.GetCommittedBefore()),
		Cursor:    req.GetPageToken(),
		Limit:     grpcapi.PageSize(req.GetPageSize()),
	})
	if err != nil {
		return nil, err
	}
	out := &pb.ListTransactionsResponse{Transactions: make([]*pb.Transaction, len(views)), NextPageToken: next}
	for i := range views {
		t := dto.TransactionFrom(&views[i])
		out.Transactions[i] = transactionOf(&t)
	}
	return out, nil
}

// GetTransaction reads a transaction by ID.
func (s *TransactionServer) GetTransaction(ctx context.Context, req *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
	v, err := s.queries.Get(ctx, req.GetLedger(), req.GetId())
	if err != nil {
		return nil, err
	}
	t := dto.TransactionFrom(&v)
	return &pb.GetTransactionResponse{Transaction: transactionOf(&t)}, nil
}

// CreateTransaction creates a transaction: one linked chain, all postings or none.
func (s *TransactionServer) CreateTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.CreateTransactionResponse, error) {
	in := make([]dto.Posting, len(req.GetPostings()))
	for i, p := range req.GetPostings() {
		in[i] = dto.Posting{
			Source: scalar.Address(p.GetSource()), Destination: scalar.Address(p.GetDestination()),
			Asset: scalar.Asset(p.GetAsset()), Amount: scalar.Amount(p.GetAmount()),
		}
	}
	postings, err := dto.ParsePostings(in)
	if err != nil {
		return nil, err
	}
	v, err := s.commands.Create(ctx, application.CreateTransactionInput{
		Ledger:       req.GetLedger(),
		Key:          req.GetIdempotencyKey(),
		RequestID:    requestid.From(ctx),
		Postings:     postings,
		Reference:    req.GetReference(),
		EventType:    req.GetEventType(),
		BusinessTime: grpcapi.TimeOf(req.GetBusinessTime()),
		Metadata:     metadataIn(req.GetMetadata()),
	})
	if err != nil {
		return nil, err
	}
	t := dto.TransactionFrom(&v)
	return &pb.CreateTransactionResponse{Transaction: transactionOf(&t)}, nil
}

// RevertTransaction reverts a transaction with one that swaps every source and destination.
func (s *TransactionServer) RevertTransaction(ctx context.Context, req *pb.RevertTransactionRequest) (*pb.RevertTransactionResponse, error) {
	v, err := s.commands.Revert(ctx, application.RevertTransactionInput{
		Ledger:    req.GetLedger(),
		Key:       req.GetIdempotencyKey(),
		RequestID: requestid.From(ctx),
		ID:        req.GetId(),
		Reference: req.GetReference(),
		Metadata:  metadataIn(req.GetMetadata()),
	})
	if err != nil {
		return nil, err
	}
	t := dto.TransactionFrom(&v)
	return &pb.RevertTransactionResponse{Transaction: transactionOf(&t)}, nil
}

// ReplaceTransactionMetadata replaces the metadata of a transaction.
func (s *TransactionServer) ReplaceTransactionMetadata(ctx context.Context, req *pb.ReplaceTransactionMetadataRequest) (*pb.ReplaceTransactionMetadataResponse, error) {
	v, err := s.commands.ReplaceMetadata(ctx, req.GetLedger(), req.GetId(), metadataIn(req.GetMetadata()))
	if err != nil {
		return nil, err
	}
	t := dto.TransactionFrom(&v)
	return &pb.ReplaceTransactionMetadataResponse{Transaction: transactionOf(&t)}, nil
}
