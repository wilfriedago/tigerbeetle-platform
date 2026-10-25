package httpapi

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

func tbTime(ts uint64) time.Time {
	return time.Unix(0, int64(ts)).UTC()
}

// postingsIn parses the postings of a request. Parsing errors are domain errors (400).
func postingsIn(in []gen.Posting) ([]ledger.Posting, error) {
	out := make([]ledger.Posting, len(in))
	for i, p := range in {
		posting, err := postingIn(p.Source, p.Destination, p.Asset, p.Amount)
		if err != nil {
			return nil, &ledger.Problem{Kind: ledger.ErrInvalid, Reason: "invalid-request", Detail: "posting " + itoa(i+1) + ": " + problemDetail(err), Posting: i}
		}
		out[i] = posting
	}
	return out, nil
}

func postingIn(source, destination, asset, amount string) (ledger.Posting, error) {
	src, err := ledger.ParseAddress(source)
	if err != nil {
		return ledger.Posting{}, err
	}
	dst, err := ledger.ParseAddress(destination)
	if err != nil {
		return ledger.Posting{}, err
	}
	a, err := ledger.ParseAsset(asset)
	if err != nil {
		return ledger.Posting{}, err
	}
	amt, err := ledger.ParseAmount(amount)
	if err != nil {
		return ledger.Posting{}, err
	}
	return ledger.Posting{Source: src, Destination: dst, Asset: a, Amount: amt}, nil
}

// ListTransactions returns a page of committed transactions.
func (s *server) ListTransactions(ctx context.Context, req gen.ListTransactionsRequestObject) (gen.ListTransactionsResponseObject, error) {
	p := req.Params
	views, next, err := s.transactions.List(ctx, app.ListInput{
		Ledger:    req.Ledger,
		Address:   deref(p.Address),
		Asset:     deref(p.Asset),
		Reference: deref(p.Reference),
		EventType: deref(p.EventType),
		After:     p.After,
		Before:    p.Before,
		Cursor:    deref(p.Cursor),
		Limit:     deref(p.Limit),
	})
	if err != nil {
		return nil, err
	}
	data := make([]gen.Transaction, len(views))
	for i := range views {
		v := &views[i]
		data[i] = transactionOf(*v)
	}
	return gen.ListTransactions200JSONResponse{Data: data, Next: optional(next)}, nil
}

// CreateTransaction applies postings as one linked chain.
func (s *server) CreateTransaction(ctx context.Context, req gen.CreateTransactionRequestObject) (gen.CreateTransactionResponseObject, error) {
	if req.Body == nil {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "a JSON body with postings is required")
	}
	postings, err := postingsIn(req.Body.Postings)
	if err != nil {
		return nil, err
	}
	v, err := s.transactions.Create(ctx, app.CreateTransactionInput{
		Ledger:       req.Ledger,
		Key:          req.Params.IdempotencyKey,
		RequestID:    RequestIDFrom(ctx),
		Postings:     postings,
		Reference:    deref(req.Body.Reference),
		EventType:    deref(req.Body.EventType),
		BusinessTime: req.Body.BusinessTime,
		Metadata:     metadataIn(req.Body.Metadata),
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateTransaction201JSONResponse(transactionOf(v)), nil
}

// GetTransaction returns a committed transaction.
func (s *server) GetTransaction(ctx context.Context, req gen.GetTransactionRequestObject) (gen.GetTransactionResponseObject, error) {
	v, err := s.transactions.Get(ctx, req.Ledger, req.Id)
	if err != nil {
		return nil, err
	}
	return gen.GetTransaction200JSONResponse(transactionOf(v)), nil
}

// ReplaceTransactionMetadata replaces the metadata of a transaction.
func (s *server) ReplaceTransactionMetadata(ctx context.Context, req gen.ReplaceTransactionMetadataRequestObject) (gen.ReplaceTransactionMetadataResponseObject, error) {
	v, err := s.transactions.ReplaceMetadata(ctx, req.Ledger, req.Id, metadataIn(req.Body))
	if err != nil {
		return nil, err
	}
	return gen.ReplaceTransactionMetadata200JSONResponse(transactionOf(v)), nil
}

// RevertTransaction creates the reversing transaction.
func (s *server) RevertTransaction(ctx context.Context, req gen.RevertTransactionRequestObject) (gen.RevertTransactionResponseObject, error) {
	var reference string
	var metadata map[string]string
	if req.Body != nil {
		reference = deref(req.Body.Reference)
		metadata = metadataIn(req.Body.Metadata)
	}
	v, err := s.transactions.Revert(ctx, req.Ledger, req.Params.IdempotencyKey, RequestIDFrom(ctx), req.Id, reference, metadata)
	if err != nil {
		return nil, err
	}
	return gen.RevertTransaction201JSONResponse(transactionOf(v)), nil
}

// ListWriteRequests returns the idempotency records of a ledger.
func (s *server) ListWriteRequests(ctx context.Context, req gen.ListWriteRequestsRequestObject) (gen.ListWriteRequestsResponseObject, error) {
	p := req.Params
	rows, next, err := s.overview.Requests(ctx, req.Ledger, string(deref(p.Status)), string(deref(p.Operation)), deref(p.Search), deref(p.Cursor), deref(p.Limit))
	if err != nil {
		return nil, err
	}
	data := make([]gen.WriteRequest, len(rows))
	for i := range rows {
		r := &rows[i]
		data[i] = writeRequestOf(*r)
	}
	return gen.ListWriteRequests200JSONResponse{Data: data, Next: optional(next)}, nil
}

// GetWriteRequest returns one idempotency record with its stored request and outcome.
func (s *server) GetWriteRequest(ctx context.Context, req gen.GetWriteRequestRequestObject) (gen.GetWriteRequestResponseObject, error) {
	rec, err := s.overview.Request(ctx, req.Ledger, req.Key)
	if err != nil {
		return nil, err
	}
	w := writeRequestOf(rec)
	body := gen.WriteRequestDetail{
		Key:            w.Key,
		Operation:      w.Operation,
		Status:         w.Status,
		CreatedAt:      w.CreatedAt,
		ResolvedAt:     w.ResolvedAt,
		TargetId:       w.TargetId,
		ProblemType:    w.ProblemType,
		ResponseStatus: w.ResponseStatus,
		RequestId:      w.RequestId,
		BodyHash:       w.BodyHash,
		Attempts:       w.Attempts,
		Request:        jsonObject(rec.Request),
		TransferIds:    make([]gen.Id, len(rec.TransferIDs)),
	}
	for i, id := range rec.TransferIDs {
		body.TransferIds[i] = id.String()
	}
	if len(rec.Response) > 0 {
		response := jsonObject(rec.Response)
		body.Response = &response
	}
	return gen.GetWriteRequest200JSONResponse(body), nil
}
