// Package kafka produces records to Kafka with franz-go (ADR-0022): an idempotent producer that
// waits for every in-sync replica (acks=all), so a record Kafka acknowledged is written once, and
// a Ping for readiness. franz-go is pure Go; segmentio/kafka-go and confluent-kafka-go were
// rejected (research (c)).
package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer writes records synchronously: Produce returns once Kafka has acknowledged the record,
// which is what lets the relay acknowledge its RabbitMQ delivery only then.
type Producer struct {
	client *kgo.Client
}

// Record is one record to produce.
type Record struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

// New connects to the seed brokers. Nothing is sent before the first Produce or Ping.
// deliveryTimeout bounds how long Produce retries a record before it returns an error, so an
// outage reaches the caller instead of blocking it.
func New(brokers []string, clientID string, deliveryTimeout time.Duration) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		// The topic is created on first use with the broker's defaults; a production cluster
		// would create it beforehand with its partitions and retention.
		kgo.AllowAutoTopicCreation(),
		kgo.RecordDeliveryTimeout(deliveryTimeout),
		kgo.ProducerLinger(5*time.Millisecond),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka: client: %w", err)
	}
	return &Producer{client: client}, nil
}

// Produce writes records in order and waits for Kafka to acknowledge every one.
func (p *Producer) Produce(ctx context.Context, records ...Record) error {
	rs := make([]*kgo.Record, len(records))
	for i, r := range records {
		rec := &kgo.Record{Topic: r.Topic, Key: r.Key, Value: r.Value}
		for k, v := range r.Headers {
			rec.Headers = append(rec.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
		}
		rs[i] = rec
	}
	if err := p.client.ProduceSync(ctx, rs...).FirstErr(); err != nil {
		return fmt.Errorf("kafka: produce: %w", err)
	}
	return nil
}

// Ping reports whether a broker answers, for readiness.
func (p *Producer) Ping(ctx context.Context) error {
	if err := p.client.Ping(ctx); err != nil {
		return fmt.Errorf("kafka: ping: %w", err)
	}
	return nil
}

// Close flushes nothing and closes the connections.
func (p *Producer) Close() {
	p.client.Close()
}
