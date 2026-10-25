package v1

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Posting moves an amount of one asset from a source to a destination.
type Posting struct {
	Source      scalar.Address `json:"source" doc:"The debited account."`
	Destination scalar.Address `json:"destination" doc:"The credited account."`
	Asset       scalar.Asset   `json:"asset"`
	Amount      scalar.Amount  `json:"amount"`
}

// PostingRecord is a committed posting with the TigerBeetle transfer behind it. Its JSON is flat;
// the spec writes it as an allOf of Posting and the extra members.
type PostingRecord struct {
	Posting
	TransferID scalar.ID `json:"transferId"`
	TbLedger   int64     `json:"tbLedger"`
}

// postingRecordExtra holds the members PostingRecord adds to Posting.
type postingRecordExtra struct {
	TransferID scalar.ID `json:"transferId"`
	TbLedger   int64     `json:"tbLedger"`
}

// Schema describes PostingRecord as Posting plus transferId and tbLedger.
func (PostingRecord) Schema(r huma.Registry) *huma.Schema {
	return component(r, "PostingRecord", func() *huma.Schema {
		return &huma.Schema{AllOf: []*huma.Schema{
			r.Schema(reflect.TypeFor[Posting](), true, ""),
			inline[postingRecordExtra](r),
		}}
	})
}

// EventTypeRef names the event type of a write and its TigerBeetle code.
type EventTypeRef struct {
	Name string `json:"name"`
	Code int    `json:"code"`
}

// Transaction is a committed transaction with its postings.
type Transaction struct {
	ID             scalar.ID         `json:"id"`
	Ledger         scalar.LedgerName `json:"ledger"`
	Postings       []PostingRecord   `json:"postings"`
	Reference      *string           `json:"reference,omitempty"`
	EventType      EventTypeRef      `json:"eventType"`
	BusinessTime   *time.Time        `json:"businessTime,omitempty"`
	Metadata       scalar.Metadata   `json:"metadata"`
	CommittedAt    time.Time         `json:"committedAt" doc:"The TigerBeetle timestamp of the chain, the only time the ledger knows."`
	Timestamp      scalar.Timestamp  `json:"timestamp"`
	Reverts        *scalar.ID        `json:"reverts,omitempty" doc:"The transaction this one reverts."`
	RevertedBy     *scalar.ID        `json:"revertedBy,omitempty" doc:"The transaction that reverted this one."`
	IdempotencyKey string            `json:"idempotencyKey"`
	RequestID      *string           `json:"requestId,omitempty"`
}

// TransformSchema describes businessTime, whose text holds backticks a struct tag cannot.
func (Transaction) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Properties["businessTime"].Description = "The client's business time, stored in `user_data_64`. Not the committed time."
	return s
}

// TransactionList is a page of transactions.
type TransactionList struct {
	Data []Transaction `json:"data"`
	Next *string       `json:"next,omitempty"`
}

// CreateTransactionRequest is the body of POST .../transactions.
type CreateTransactionRequest struct {
	Postings     []Posting       `json:"postings" minItems:"1" maxItems:"64"`
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

// WriteRequest is an idempotency record: a write with its key, operation and outcome.
type WriteRequest struct {
	Key            string               `json:"key"`
	Operation      scalar.Operation     `json:"operation"`
	Status         scalar.RequestStatus `json:"status"`
	CreatedAt      time.Time            `json:"createdAt"`
	ResolvedAt     *time.Time           `json:"resolvedAt,omitempty"`
	TargetID       scalar.ID            `json:"targetId" doc:"The transaction or hold minted for this write."`
	ProblemType    *string              `json:"problemType,omitempty" doc:"The problem type of a declined or failed write."`
	ResponseStatus *int                 `json:"responseStatus,omitempty"`
	RequestID      *string              `json:"requestId,omitempty"`
	BodyHash       string               `json:"bodyHash" doc:"SHA-256 of the canonical request body, hex."`
	Attempts       int                  `json:"attempts" doc:"TigerBeetle submissions, including reconciler retries."`
}

// WriteRequestList is a page of write requests.
type WriteRequestList struct {
	Data []WriteRequest `json:"data"`
	Next *string        `json:"next,omitempty"`
}

// WriteRequestDetail is a write request with its stored request, outcome and minted IDs.
type WriteRequestDetail struct {
	WriteRequest
	Request     map[string]any `json:"request"`
	Response    map[string]any `json:"response,omitempty"`
	TransferIDs []scalar.ID    `json:"transferIds"`
}

// Schema describes WriteRequestDetail as WriteRequest plus the stored request and outcome. The
// extra part is written by hand: the stored bodies are free-form objects.
func (WriteRequestDetail) Schema(r huma.Registry) *huma.Schema {
	return component(r, "WriteRequestDetail", func() *huma.Schema {
		return &huma.Schema{AllOf: []*huma.Schema{
			r.Schema(reflect.TypeFor[WriteRequest](), true, ""),
			{
				Type:     huma.TypeObject,
				Required: []string{"request", "transferIds"},
				Properties: map[string]*huma.Schema{
					"request": {Type: huma.TypeObject, Description: "The canonical request body as stored.", AdditionalProperties: true},
					"response": {
						Type: huma.TypeObject, Description: "The stored response or problem body.", AdditionalProperties: true,
					},
					"transferIds": {
						Type: huma.TypeArray, Description: "The transfer IDs minted before the first submission, in chain order.",
						Items: scalar.ID("").Schema(r),
					},
				},
			},
		}}
	})
}

func registerTransactions(api huma.API, h *handlers) {
	tags := []string{"transactions"}
	restapi.Register(api, huma.Operation{
		OperationID: "listTransactions", Method: http.MethodGet, Path: "/v1/ledgers/{ledger}/transactions", Tags: tags,
		Summary: "List transactions",
		Description: "Committed transactions, newest first, with their postings read from TigerBeetle. Declined\n" +
			"writes are not transactions in TigerBeetle; they are listed under write requests. There is\n" +
			"no sort or filter by amount (ADR-0004).\n",
		Parameters: []*huma.Param{cursorParam()},
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
		Parameters: []*huma.Param{cursorParam()},
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
	LedgerPath
	Address   scalar.AddressPrefix `query:"address" doc:"Only transactions with a posting from or to an account at or below this prefix."`
	Asset     scalar.Asset         `query:"asset"`
	Reference string               `query:"reference" maxLength:"255"`
	EventType string               `query:"eventType" maxLength:"63"`
	After     time.Time            `query:"after" doc:"Committed at or after this time."`
	Before    time.Time            `query:"before" doc:"Committed before this time."`
	Page
},
) (*body[TransactionList], error) {
	views, next, err := h.Transactions.List(ctx, app.ListInput{
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
	out := TransactionList{Data: make([]Transaction, len(views)), Next: optional(next)}
	for i := range views {
		out.Data[i] = transactionOf(&views[i])
	}
	return &body[TransactionList]{Body: out}, nil
}

func (h *handlers) createTransaction(ctx context.Context, in *struct {
	LedgerPath
	WriteKey
	Body CreateTransactionRequest
},
) (*body[Transaction], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	postings, err := postingsIn(in.Body.Postings)
	if err != nil {
		return nil, err
	}
	v, err := h.Transactions.Create(ctx, app.CreateTransactionInput{
		Ledger:       string(in.Ledger),
		Key:          in.IdempotencyKey,
		RequestID:    requestid.From(ctx),
		Postings:     postings,
		Reference:    deref(in.Body.Reference),
		EventType:    deref(in.Body.EventType),
		BusinessTime: in.Body.BusinessTime,
		Metadata:     metadataIn(in.Body.Metadata),
	})
	if err != nil {
		return nil, err
	}
	return &body[Transaction]{Body: transactionOf(&v)}, nil
}

func (h *handlers) getTransaction(ctx context.Context, in *struct {
	LedgerPath
	ResourcePath
},
) (*body[Transaction], error) {
	v, err := h.Transactions.Get(ctx, string(in.Ledger), string(in.ID))
	if err != nil {
		return nil, err
	}
	return &body[Transaction]{Body: transactionOf(&v)}, nil
}

func (h *handlers) replaceTransactionMetadata(ctx context.Context, in *struct {
	LedgerPath
	ResourcePath
	Body scalar.Metadata
},
) (*body[Transaction], error) {
	v, err := h.Transactions.ReplaceMetadata(ctx, string(in.Ledger), string(in.ID), metadataIn(in.Body))
	if err != nil {
		return nil, err
	}
	return &body[Transaction]{Body: transactionOf(&v)}, nil
}

func (h *handlers) revertTransaction(ctx context.Context, in *struct {
	LedgerPath
	ResourcePath
	WriteKey
	Body *RevertTransactionRequest
},
) (*body[Transaction], error) {
	if err := requireIdempotencyKey(in.IdempotencyKey); err != nil {
		return nil, err
	}
	var reference string
	var metadata map[string]string
	if in.Body != nil {
		reference = deref(in.Body.Reference)
		metadata = metadataIn(in.Body.Metadata)
	}
	v, err := h.Transactions.Revert(ctx, string(in.Ledger), in.IdempotencyKey, requestid.From(ctx), string(in.ID), reference, metadata)
	if err != nil {
		return nil, err
	}
	return &body[Transaction]{Body: transactionOf(&v)}, nil
}

func (h *handlers) listWriteRequests(ctx context.Context, in *struct {
	LedgerPath
	Status    scalar.RequestStatus `query:"status"`
	Operation scalar.Operation     `query:"operation"`
	Search    string               `query:"search" maxLength:"255" doc:"An Idempotency-Key, or the start of one, or a request ID."`
	Page
},
) (*body[WriteRequestList], error) {
	rows, next, err := h.Overview.Requests(ctx, string(in.Ledger), string(in.Status), string(in.Operation), in.Search, in.Cursor, in.Limit)
	if err != nil {
		return nil, err
	}
	out := WriteRequestList{Data: make([]WriteRequest, len(rows)), Next: optional(next)}
	for i := range rows {
		out.Data[i] = writeRequestOf(&rows[i])
	}
	return &body[WriteRequestList]{Body: out}, nil
}

func (h *handlers) getWriteRequest(ctx context.Context, in *struct {
	LedgerPath
	Key string `path:"key" minLength:"1" maxLength:"255" doc:"The Idempotency-Key."`
},
) (*body[WriteRequestDetail], error) {
	rec, err := h.Overview.Request(ctx, string(in.Ledger), in.Key)
	if err != nil {
		return nil, err
	}
	out := WriteRequestDetail{
		WriteRequest: writeRequestOf(&rec),
		Request:      jsonObject(rec.Request),
		TransferIDs:  make([]scalar.ID, len(rec.TransferIDs)),
	}
	for i, id := range rec.TransferIDs {
		out.TransferIDs[i] = scalar.ID(id.String())
	}
	if len(rec.Response) > 0 {
		out.Response = jsonObject(rec.Response)
	}
	return &body[WriteRequestDetail]{Body: out}, nil
}

// postingsIn parses the postings of a request. A posting that does not parse is an invalid request
// that names the posting.
func postingsIn(in []Posting) ([]ledger.Posting, error) {
	out := make([]ledger.Posting, len(in))
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

func postingIn(source, destination scalar.Address, asset scalar.Asset, amount scalar.Amount) (ledger.Posting, error) {
	src, err := ledger.ParseAddress(string(source))
	if err != nil {
		return ledger.Posting{}, err
	}
	dst, err := ledger.ParseAddress(string(destination))
	if err != nil {
		return ledger.Posting{}, err
	}
	a, err := ledger.ParseAsset(string(asset))
	if err != nil {
		return ledger.Posting{}, err
	}
	amt, err := ledger.ParseAmount(string(amount))
	if err != nil {
		return ledger.Posting{}, err
	}
	return ledger.Posting{Source: src, Destination: dst, Asset: a, Amount: amt}, nil
}

func eventTypeRefOf(e ledger.EventType) EventTypeRef {
	return EventTypeRef{Name: e.Name, Code: int(e.Code)}
}

func transactionOf(v *app.TransactionView) Transaction {
	postings := make([]PostingRecord, len(v.Postings))
	for i, p := range v.Postings {
		postings[i] = PostingRecord{
			Posting: Posting{
				Source: scalar.Address(p.Source), Destination: scalar.Address(p.Destination),
				Asset: scalar.Asset(p.Asset), Amount: scalar.Amount(p.Amount.String()),
			},
			TransferID: scalar.ID(p.TransferID.String()),
			TbLedger:   int64(p.TBLedger),
		}
	}
	return Transaction{
		ID:             scalar.ID(v.ID.String()),
		Ledger:         scalar.LedgerName(v.Ledger),
		Postings:       postings,
		Reference:      optional(v.Reference),
		EventType:      eventTypeRefOf(v.EventType),
		BusinessTime:   v.BusinessTime,
		Metadata:       metadataOf(v.Metadata),
		CommittedAt:    v.CommittedAt,
		Timestamp:      timestampOf(v.Timestamp),
		Reverts:        optionalID(v.Reverts),
		RevertedBy:     optionalID(v.RevertedBy),
		IdempotencyKey: v.IdempotencyKey,
		RequestID:      optional(v.RequestID),
	}
}

func writeRequestOf(rec *app.WriteRecord) WriteRequest {
	w := WriteRequest{
		Key:         rec.Key,
		Operation:   scalar.Operation(rec.Operation),
		Status:      scalar.RequestStatus(rec.Status),
		CreatedAt:   rec.CreatedAt,
		ResolvedAt:  rec.ResolvedAt,
		TargetID:    scalar.ID(rec.TargetID.String()),
		ProblemType: optional(problem.TypeOf(rec.ProblemType)),
		RequestID:   optional(rec.RequestID),
		BodyHash:    hex.EncodeToString(rec.BodyHash),
		Attempts:    rec.Attempts,
	}
	if rec.ResponseStatus != 0 {
		status := rec.ResponseStatus
		w.ResponseStatus = &status
	}
	return w
}

// jsonObject decodes a stored JSON object for display; anything else becomes an empty object.
func jsonObject(raw []byte) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func optionalID(id *ledger.ID) *scalar.ID {
	if id == nil {
		return nil
	}
	s := scalar.ID(id.String())
	return &s
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
