package v1

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// TransactionList is a page of transactions.
type TransactionList struct {
	Data []dto.Transaction `json:"data"`
	Next *string           `json:"next,omitempty"`
}

// CreateTransactionRequest is the body of POST .../transactions.
type CreateTransactionRequest struct {
	Postings     []dto.Posting   `json:"postings" minItems:"1" maxItems:"64"`
	Reference    *string         `json:"reference,omitempty" maxLength:"255"`
	EventType    *string         `json:"eventType,omitempty" maxLength:"63" doc:"An event type name of the ledger; the default event type when absent."`
	BusinessTime *time.Time      `json:"businessTime,omitempty"`
	Metadata     scalar.Metadata `json:"metadata,omitempty"`
}

// RevertTransactionRequest is the optional body of POST .../revert.
type RevertTransactionRequest struct {
	Reference *string         `json:"reference,omitempty" maxLength:"255"`
	Metadata  scalar.Metadata `json:"metadata,omitempty"`
}

// WriteRequestList is a page of write requests.
type WriteRequestList struct {
	Data []dto.WriteRequest `json:"data"`
	Next *string            `json:"next,omitempty"`
}

func registerTransactions(api huma.API, h *handlers) {
	tags := []string{"transactions"}
	restapi.Register(api, huma.Operation{
		OperationID: "listTransactions", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/transactions", Tags: tags,
		Summary: "List transactions",
		Description: "Committed transactions, newest first, with their postings read from TigerBeetle. Declined\n" +
			"writes are not transactions in TigerBeetle; they are listed under write requests. There is\n" +
			"no sort or filter by amount (ADR-0004).\n",
		Parameters: []*huma.Param{restapi.CursorParam()},
		Responses:  restapi.Success(http.StatusOK, "A page of transactions."),
	}, h.listTransactions)

	restapi.Register(api, huma.Operation{
		OperationID: "createTransaction", Method: http.MethodPost, Path: "/v1/ledgers/{ledger}/transactions", Tags: tags,
		Summary: "Create a transaction",
		Description: "Applies the postings as one linked chain of TigerBeetle transfers: all of them or none.\n" +
			"Accounts are created by their first posting with the flags of the matching account rule\n" +
			"(ADR-0006). A decline, such as `exceeds_credits`, answers 422 with the result of every\n" +
			"posting in `results`; nothing was applied.\n",
		Parameters:    []*huma.Param{idempotencyKeyParam()},
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The transaction was committed."),
	}, h.createTransaction)

	restapi.Register(api, huma.Operation{
		OperationID: "getTransaction", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/transactions/{id}", Tags: tags,
		Summary:     "Get a transaction",
		Description: "The transaction with its postings and the TigerBeetle transfer behind each one.",
		Responses:   restapi.Success(http.StatusOK, "The transaction."),
	}, h.getTransaction)

	restapi.Register(api, huma.Operation{
		OperationID: "replaceTransactionMetadata", Method: http.MethodPut, Path: "/v1/ledgers/{ledger}/transactions/{id}/metadata", Tags: tags,
		Summary:     "Replace transaction metadata",
		Description: "Replaces the metadata stored in Postgres with the transaction. The postings never change.",
		Responses:   restapi.Success(http.StatusOK, "The transaction with its new metadata."),
	}, h.replaceTransactionMetadata)

	restapi.Register(api, huma.Operation{
		OperationID: "revertTransaction", Method: http.MethodPost, Path: "/v1/ledgers/{ledger}/transactions/{id}/revert", Tags: tags,
		Summary: "Revert a transaction",
		Description: "Creates a new transaction with every posting reversed. It is a normal linked chain and can\n" +
			"be declined, for example when a destination has already spent the funds; there is no\n" +
			"force option, because TigerBeetle flags and balance limits are immutable (ADR-0004). A\n" +
			"transaction is reverted at most once (409 `already-reverted`).\n",
		Parameters:    []*huma.Param{idempotencyKeyParam()},
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The reverting transaction was committed."),
	}, h.revertTransaction)

	restapi.Register(api, huma.Operation{
		OperationID: "listWriteRequests", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/requests", Tags: tags,
		Summary: "List write requests",
		Description: "The idempotency records of the ledger, newest first: every write with its key, operation\n" +
			"and outcome. This is the only place where declined and in-flight writes are visible.\n",
		Parameters: []*huma.Param{restapi.CursorParam()},
		Responses:  restapi.Success(http.StatusOK, "A page of write requests."),
	}, h.listWriteRequests)

	restapi.Register(api, huma.Operation{
		OperationID: "getWriteRequest", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/requests/{key}", Tags: tags,
		Summary:     "Get a write request",
		Description: "The record with the stored request, the stored outcome and the minted IDs.",
		Responses:   restapi.Success(http.StatusOK, "The write request."),
	}, h.getWriteRequest)
}

func (h *handlers) listTransactions(ctx context.Context, in *struct {
	restapi.LedgerPath
	Address   scalar.AddressPrefix `query:"address" doc:"Only transactions with a posting from or to an account at or below this prefix."`
	Asset     scalar.Asset         `query:"asset"`
	Reference string               `query:"reference" maxLength:"255"`
	EventType string               `query:"eventType" maxLength:"63"`
	After     time.Time            `query:"after" doc:"Committed at or after this time."`
	Before    time.Time            `query:"before" doc:"Committed before this time."`
	restapi.Page
},
) (*restapi.Body[TransactionList], error) {
	views, next, err := h.TransactionQueries.List(ctx, application.ListTransactionsInput{
		Ledger:    string(in.Ledger),
		Address:   string(in.Address),
		Asset:     string(in.Asset),
		Reference: in.Reference,
		EventType: in.EventType,
		After:     optionalTime(in.After),
		Before:    optionalTime(in.Before),
		Cursor:    in.Cursor,
		Limit:     in.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := TransactionList{Data: make([]dto.Transaction, len(views)), Next: restapi.Optional(next)}
	for i := range views {
		out.Data[i] = dto.TransactionFrom(&views[i])
	}
	return &restapi.Body[TransactionList]{Body: out}, nil
}

func (h *handlers) createTransaction(ctx context.Context, in *struct {
	restapi.LedgerPath
	WriteKey
	Body CreateTransactionRequest
},
) (*restapi.Body[dto.Transaction], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	postings, err := postingsIn(in.Body.Postings)
	if err != nil {
		return nil, err
	}
	v, err := h.TransactionCommands.Create(ctx, application.CreateTransactionInput{
		Ledger:       string(in.Ledger),
		Key:          in.IdempotencyKey,
		RequestID:    requestid.From(ctx),
		Postings:     postings,
		Reference:    deref(in.Body.Reference),
		EventType:    deref(in.Body.EventType),
		BusinessTime: in.Body.BusinessTime,
		Metadata:     in.Body.Metadata.Map(),
	})
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Transaction]{Body: dto.TransactionFrom(&v)}, nil
}

func (h *handlers) getTransaction(ctx context.Context, in *struct {
	restapi.LedgerPath
	ResourcePath
},
) (*restapi.Body[dto.Transaction], error) {
	v, err := h.TransactionQueries.Get(ctx, string(in.Ledger), string(in.ID))
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Transaction]{Body: dto.TransactionFrom(&v)}, nil
}

func (h *handlers) replaceTransactionMetadata(ctx context.Context, in *struct {
	restapi.LedgerPath
	ResourcePath
	Body scalar.Metadata
},
) (*restapi.Body[dto.Transaction], error) {
	v, err := h.TransactionCommands.ReplaceMetadata(ctx, string(in.Ledger), string(in.ID), in.Body.Map())
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Transaction]{Body: dto.TransactionFrom(&v)}, nil
}

func (h *handlers) revertTransaction(ctx context.Context, in *struct {
	restapi.LedgerPath
	ResourcePath
	WriteKey
	Body *RevertTransactionRequest
},
) (*restapi.Body[dto.Transaction], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	revert := application.RevertTransactionInput{Ledger: string(in.Ledger), Key: in.IdempotencyKey, RequestID: requestid.From(ctx), ID: string(in.ID)}
	if in.Body != nil {
		revert.Reference = deref(in.Body.Reference)
		revert.Metadata = in.Body.Metadata.Map()
	}
	v, err := h.TransactionCommands.Revert(ctx, revert)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.Transaction]{Body: dto.TransactionFrom(&v)}, nil
}

func (h *handlers) listWriteRequests(ctx context.Context, in *struct {
	restapi.LedgerPath
	Status    scalar.RequestStatus `query:"status"`
	Operation scalar.Operation     `query:"operation"`
	Search    string               `query:"search" maxLength:"255" doc:"An Idempotency-Key, or the start of one, or a request ID."`
	restapi.Page
},
) (*restapi.Body[WriteRequestList], error) {
	rows, next, err := h.RequestQueries.List(ctx, application.ListRequestsInput{
		Ledger: string(in.Ledger), Status: string(in.Status), Operation: string(in.Operation), Search: in.Search, Cursor: in.Cursor, Limit: in.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := WriteRequestList{Data: make([]dto.WriteRequest, len(rows)), Next: restapi.Optional(next)}
	for i := range rows {
		out.Data[i] = dto.WriteRequestFrom(&rows[i])
	}
	return &restapi.Body[WriteRequestList]{Body: out}, nil
}

func (h *handlers) getWriteRequest(ctx context.Context, in *struct {
	restapi.LedgerPath
	Key string `path:"key" minLength:"1" maxLength:"255" doc:"The Idempotency-Key."`
},
) (*restapi.Body[dto.WriteRequestDetail], error) {
	rec, err := h.RequestQueries.Get(ctx, string(in.Ledger), in.Key)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.WriteRequestDetail]{Body: dto.WriteRequestDetailFrom(&rec)}, nil
}

// postingsIn parses the postings of a request. A posting that does not parse is an invalid request
// that names the posting.
func postingsIn(in []dto.Posting) ([]domain.Posting, error) {
	out := make([]domain.Posting, len(in))
	for i, p := range in {
		posting, err := postingIn(p.Source, p.Destination, p.Asset, p.Amount)
		if err != nil {
			return nil, &ledger.Problem{
				Kind: ledger.ErrInvalid, Reason: "invalid-request", Posting: i,
				Detail: "posting " + strconv.Itoa(i+1) + ": " + problem.From(err).Detail,
			}
		}
		out[i] = posting
	}
	return out, nil
}

func postingIn(source, destination scalar.Address, asset scalar.Asset, amount scalar.Amount) (domain.Posting, error) {
	src, err := ledger.ParseAddress(string(source))
	if err != nil {
		return domain.Posting{}, err
	}
	dst, err := ledger.ParseAddress(string(destination))
	if err != nil {
		return domain.Posting{}, err
	}
	a, err := ledger.ParseAsset(string(asset))
	if err != nil {
		return domain.Posting{}, err
	}
	amt, err := ledger.ParseAmount(string(amount))
	if err != nil {
		return domain.Posting{}, err
	}
	return domain.Posting{Source: src, Destination: dst, Asset: a, Amount: amt}, nil
}

// optionalTime returns nil for the zero time: an absent query parameter.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
