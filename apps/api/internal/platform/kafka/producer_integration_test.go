//go:build integration

package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
)

// TestProduce is spike S4 of the plan: the pinned apache/kafka image as a generic container with
// a fixed port and advertised listener, a record produced with acks=all and read back.
func TestProduce(t *testing.T) {
	start := time.Now()
	broker := devstack.StartKafka(t)
	t.Logf("kafka ready in %v", time.Since(start).Round(time.Millisecond))

	p, err := New([]string{broker}, "test", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := p.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Produce(ctx, Record{Topic: "tbp.test", Key: []byte("k"), Value: []byte(`{"n":"1"}`), Headers: map[string]string{"event-type": "test"}}); err != nil {
		t.Fatal(err)
	}

	consumer, err := kgo.NewClient(kgo.SeedBrokers(broker), kgo.ConsumeTopics("tbp.test"), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	fetches := consumer.PollFetches(ctx)
	if errs := fetches.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}
	records := fetches.Records()
	if len(records) != 1 || string(records[0].Key) != "k" || string(records[0].Value) != `{"n":"1"}` ||
		len(records[0].Headers) != 1 || string(records[0].Headers[0].Value) != "test" {
		t.Fatalf("records = %+v", records)
	}
}
