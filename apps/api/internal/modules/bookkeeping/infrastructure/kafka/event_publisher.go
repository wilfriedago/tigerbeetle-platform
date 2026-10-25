// Package kafka publishes ledger events to Kafka (ADR-0022), implementing the domain's
// LedgerEventPublisher with internal/platform/kafka.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	platformkafka "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/kafka"
)

// EventPublisher writes one record per ledger event to the events topic.
type EventPublisher struct {
	producer *platformkafka.Producer
	topic    string
}

// NewEventPublisher publishes to topic, tbp.ledger-events.v1 by default (KAFKA_EVENTS_TOPIC).
func NewEventPublisher(producer *platformkafka.Producer, topic string) *EventPublisher {
	return &EventPublisher{producer: producer, topic: topic}
}

// Publish writes the events in order and returns once Kafka has acknowledged them all. The key
// is the API transaction ID (user_data_128), or the transfer ID when that is zero, so the legs of
// one transaction share a partition and keep their order. Headers carry event-type and
// tb-timestamp; tb-timestamp is unique per event, so consumers deduplicate on it.
func (p *EventPublisher) Publish(ctx context.Context, events ...domain.LedgerEvent) error {
	records := make([]platformkafka.Record, len(events))
	for i := range events {
		ev := &events[i]
		value, err := json.Marshal(RecordOf(ev))
		if err != nil {
			return fmt.Errorf("kafka: encode event %d: %w", ev.Timestamp, err)
		}
		key := ev.Transfer.UserData128
		if key.IsZero() {
			key = ev.Transfer.ID
		}
		records[i] = platformkafka.Record{
			Topic: p.topic, Key: []byte(key.String()), Value: value,
			Headers: map[string]string{"event-type": ev.Type, "tb-timestamp": strconv.FormatUint(ev.Timestamp, 10)},
		}
	}
	return p.producer.Produce(ctx, records...)
}

// Record is the JSON value of a ledger event record (docs/ledger-events.md). Every u128 and u64
// is a string: IDs as 32 hexadecimal characters like everywhere in the API, amounts, balances,
// user_data_64 and timestamps in decimal. u32 and u16 values are numbers.
type Record struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	// Ledger and Asset are null for a transfer this platform does not know, such as one written
	// by another TigerBeetle client.
	Ledger        *string        `json:"ledger"`
	Asset         *string        `json:"asset"`
	TBLedger      uint32         `json:"tbLedger"`
	TransactionID *string        `json:"transactionId"`
	Transfer      TransferRecord `json:"transfer"`
	DebitAccount  AccountRecord  `json:"debitAccount"`
	CreditAccount AccountRecord  `json:"creditAccount"`
}

// TransferRecord is the transfer of a ledger event.
type TransferRecord struct {
	ID          string  `json:"id"`
	Amount      string  `json:"amount"`
	PendingID   *string `json:"pendingId"`
	UserData128 string  `json:"userData128"`
	UserData64  string  `json:"userData64"`
	UserData32  uint32  `json:"userData32"`
	Timeout     uint32  `json:"timeout"`
	Code        uint16  `json:"code"`
	Flags       uint16  `json:"flags"`
	Timestamp   string  `json:"timestamp"`
}

// AccountRecord is an account of a ledger event with its balances as of the event.
type AccountRecord struct {
	ID             string  `json:"id"`
	Address        *string `json:"address"`
	DebitsPending  string  `json:"debitsPending"`
	DebitsPosted   string  `json:"debitsPosted"`
	CreditsPending string  `json:"creditsPending"`
	CreditsPosted  string  `json:"creditsPosted"`
	UserData128    string  `json:"userData128"`
	UserData64     string  `json:"userData64"`
	UserData32     uint32  `json:"userData32"`
	Code           uint16  `json:"code"`
	Flags          uint16  `json:"flags"`
	Timestamp      string  `json:"timestamp"`
}

// RecordOf renders a ledger event.
func RecordOf(ev *domain.LedgerEvent) Record {
	t := ev.Transfer
	return Record{
		Type:          ev.Type,
		Timestamp:     u64(ev.Timestamp),
		Ledger:        optional(ev.Ledger),
		Asset:         optional(ev.Asset),
		TBLedger:      ev.TBLedger,
		TransactionID: optionalID(t.UserData128),
		Transfer: TransferRecord{
			ID: t.ID.String(), Amount: t.Amount.String(), PendingID: optionalID(t.PendingID),
			UserData128: t.UserData128.String(), UserData64: u64(t.UserData64), UserData32: t.UserData32,
			Timeout: t.Timeout, Code: t.Code, Flags: t.Flags, Timestamp: u64(t.Timestamp),
		},
		DebitAccount:  accountOf(&ev.Debit, ev.DebitAddress),
		CreditAccount: accountOf(&ev.Credit, ev.CreditAddress),
	}
}

func accountOf(a *domain.ChangeAccount, address string) AccountRecord {
	return AccountRecord{
		ID: a.ID.String(), Address: optional(address),
		DebitsPending: a.DebitsPending.String(), DebitsPosted: a.DebitsPosted.String(),
		CreditsPending: a.CreditsPending.String(), CreditsPosted: a.CreditsPosted.String(),
		UserData128: a.UserData128.String(), UserData64: u64(a.UserData64), UserData32: a.UserData32,
		Code: a.Code, Flags: a.Flags, Timestamp: u64(a.Timestamp),
	}
}

func u64(v uint64) string { return strconv.FormatUint(v, 10) }

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalID(id ledger.ID) *string {
	if id.IsZero() {
		return nil
	}
	s := id.String()
	return &s
}
