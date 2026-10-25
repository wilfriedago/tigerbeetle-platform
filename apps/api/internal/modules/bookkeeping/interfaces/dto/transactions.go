package dto

import (
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
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
	return restapi.Component(r, "PostingRecord", func() *huma.Schema {
		return &huma.Schema{AllOf: []*huma.Schema{
			r.Schema(reflect.TypeFor[Posting](), true, ""),
			restapi.Inline[postingRecordExtra](r),
		}}
	})
}

// EventTypeRef names the event type of a write and its TigerBeetle code.
type EventTypeRef struct {
	Name string `json:"name"`
	Code int    `json:"code"`
}

// EventTypeRefOf renders the event type of a write.
func EventTypeRefOf(e ledger.EventType) EventTypeRef {
	return EventTypeRef{Name: e.Name, Code: int(e.Code)}
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

// TransactionFrom renders a committed transaction.
func TransactionFrom(v *application.TransactionView) Transaction {
	postings := make([]PostingRecord, len(v.Postings))
	for i := range v.Postings {
		p := &v.Postings[i]
		postings[i] = PostingRecord{
			Posting: Posting{
				Source: scalar.Address(p.Source), Destination: scalar.Address(p.Destination),
				Asset: scalar.Asset(p.Asset), Amount: scalar.AmountOf(p.Amount),
			},
			TransferID: scalar.IDOf(p.TransferID),
			TbLedger:   int64(p.TBLedger),
		}
	}
	return Transaction{
		ID:             scalar.IDOf(v.ID),
		Ledger:         scalar.LedgerName(v.Ledger),
		Postings:       postings,
		Reference:      restapi.Optional(v.Reference),
		EventType:      EventTypeRefOf(v.EventType),
		BusinessTime:   v.BusinessTime,
		Metadata:       scalar.MetadataOf(v.Metadata),
		CommittedAt:    v.CommittedAt,
		Timestamp:      scalar.TimestampOf(v.Timestamp),
		Reverts:        scalar.OptionalID(v.Reverts),
		RevertedBy:     scalar.OptionalID(v.RevertedBy),
		IdempotencyKey: v.IdempotencyKey,
		RequestID:      restapi.Optional(v.RequestID),
	}
}
