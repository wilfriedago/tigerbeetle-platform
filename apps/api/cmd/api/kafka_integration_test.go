//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/bootstrap"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/devstack"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/config"
)

const eventsTopic = "tbp.ledger-events.v1"

// eventRecord is the part of a ledger event record the test reads.
type eventRecord struct {
	key, eventType, tbTimestamp string
	value                       struct {
		Type          string
		Ledger        *string
		Asset         *string
		TransactionID *string
		Transfer      struct{ ID, Amount string }
		DebitAccount  struct{ Address *string }
		CreditAccount struct{ Address *string }
	}
}

// relayApp is one API process with the relay on.
type relayApp struct {
	app    *bootstrap.App
	cancel context.CancelFunc
	done   chan error
}

func startRelayApp(t *testing.T, cfg config.Config) *relayApp {
	t.Helper()
	app, err := bootstrap.Build(context.Background(), cfg, bootstrap.Service{Name: serviceName, Version: version},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &relayApp{app: app, cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- app.Run(ctx) }()
	return r
}

func (r *relayApp) stop(t *testing.T) {
	t.Helper()
	r.cancel()
	if err := <-r.done; err != nil {
		t.Errorf("run: %v", err)
	}
}

// readEvents reads the topic from the start until want distinct tb-timestamps arrived or the
// deadline passed, and returns every record, duplicates included.
func readEvents(t *testing.T, broker string, want int, deadline time.Duration) []eventRecord {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(broker), kgo.ConsumeTopics(eventsTopic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	var out []eventRecord
	seen := map[string]bool{}
	for len(seen) < want {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			break
		}
		fetches.EachRecord(func(rec *kgo.Record) {
			var e eventRecord
			e.key = string(rec.Key)
			for _, h := range rec.Headers {
				switch h.Key {
				case "event-type":
					e.eventType = string(h.Value)
				case "tb-timestamp":
					e.tbTimestamp = string(h.Value)
				}
			}
			if err := json.Unmarshal(rec.Value, &e.value); err != nil {
				t.Errorf("record %s: %v", rec.Value, err)
			}
			seen[e.tbTimestamp] = true
			out = append(out, e)
		})
	}
	return out
}

// TestLedgerEventsToKafka runs the whole path of ADR-0022 against real containers: TigerBeetle,
// Postgres, RabbitMQ, the CDC job of the pinned image and Kafka, with the relay in the API.
func TestLedgerEventsToKafka(t *testing.T) {
	tbHost, tbNetwork := devstack.StartTigerBeetleNode(t)
	amqpURL, amqpNetwork := devstack.StartRabbitMQNode(t)
	broker := devstack.StartKafka(t)
	cfg := config.Config{
		Addr:                  "127.0.0.1:0",
		PostgresURL:           devstack.StartPostgres(t),
		TigerBeetleAddresses:  []string{tbHost},
		MigrateOnStart:        true,
		RESTEnabled:           true,
		ShutdownTimeout:       5 * time.Second,
		ReadinessTimeout:      2 * time.Second,
		TigerBeetleTimeout:    10 * time.Second,
		ReconcilerGracePeriod: time.Minute,
		ReconcilerInterval:    time.Hour,
		KafkaEventsEnabled:    true,
		KafkaBrokers:          []string{broker},
		KafkaEventsTopic:      eventsTopic,
		CDCAMQPURL:            amqpURL,
		CDCQueue:              "tbp.ledger-events.relay",
	}
	auth := trustTestIssuer(t, &cfg)
	// Writes go through their own process, with the relay off, so stopping a relay process does
	// not stop the writes, as with two replicas.
	writerCfg := cfg
	writerCfg.KafkaEventsEnabled = false
	writer, err := bootstrap.Build(context.Background(), writerCfg, bootstrap.Service{Name: serviceName, Version: version},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("build writer: %v", err)
	}
	t.Cleanup(writer.Close)
	h := &harness{t: t, app: writer, auth: auth}
	auth.ownDefault(t, cfg.PostgresURL)
	h.server = httptest.NewServer(auth.withCredentials(writer.Handler()))
	t.Cleanup(h.server.Close)

	first := startRelayApp(t, cfg)
	// The relay declares the exchange and queue when it connects; the CDC job needs them before
	// its first publish.
	waitFor(t, 30*time.Second, func() bool {
		rec := httptest.NewRecorder()
		first.app.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
		var body struct {
			Dependencies map[string]struct{ Status string }
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return body.Dependencies["rabbitmq"].Status == "up"
	})
	cdc := devstack.StartCDC(t, tbNetwork, amqpNetwork, "tbp.cdc")

	h.expect(h.do(http.MethodPost, "/v1/ledgers/events", "", map[string]any{}), http.StatusCreated, "create ledger")
	tx := h.do(http.MethodPost, "/v1/ledgers/events/transactions", "kafka-1", map[string]any{"postings": []any{
		posting("world", "users:1:wallet", "EUR/2", "1250"),
		posting("users:1:wallet", "fees:card", "EUR/2", "50"),
	}})
	h.expect(tx, http.StatusCreated, "transaction")
	txID := tx.str("id")

	t.Run("one record per leg, amounts as strings, keyed by the transaction", func(t *testing.T) {
		records := readEvents(t, broker, 2, time.Minute)
		if len(records) < 2 {
			t.Fatalf("%d records, want 2", len(records))
		}
		wants := []struct{ amount, debit, credit string }{{"1250", "world", "users:1:wallet"}, {"50", "users:1:wallet", "fees:card"}}
		for i, want := range wants {
			r := records[i]
			v := r.value
			if r.key != txID || r.eventType != "single_phase" || v.Type != "single_phase" || r.tbTimestamp == "" ||
				v.TransactionID == nil || *v.TransactionID != txID || v.Transfer.Amount != want.amount ||
				v.Ledger == nil || *v.Ledger != "events" || v.Asset == nil || *v.Asset != "EUR/2" ||
				v.DebitAccount.Address == nil || *v.DebitAccount.Address != want.debit ||
				v.CreditAccount.Address == nil || *v.CreditAccount.Address != want.credit {
				t.Errorf("leg %d = %+v", i+1, r)
			}
		}
	})

	t.Run("an expiry is published with no request behind it", func(t *testing.T) {
		hold := h.do(http.MethodPost, "/v1/ledgers/events/holds", "kafka-hold", map[string]any{
			"source": "users:1:wallet", "destination": "fees:card", "asset": "EUR/2", "amount": "10", "timeoutSeconds": 1,
		})
		h.expect(hold, http.StatusCreated, "hold")
		found := false
		for _, r := range readEvents(t, broker, 4, time.Minute) {
			if r.eventType == "two_phase_expired" && r.value.TransactionID != nil && *r.value.TransactionID == hold.str("id") {
				found = true
			}
		}
		if !found {
			t.Fatal("no two_phase_expired record for the hold")
		}
	})

	t.Run("a relay and a CDC job stopped mid-stream give duplicates, never gaps", func(t *testing.T) {
		const writes = 30
		for i := range writes {
			r := h.do(http.MethodPost, "/v1/ledgers/events/transactions", fmt.Sprintf("burst-%d", i), map[string]any{
				"postings": []any{posting("world", "users:2:wallet", "EUR/2", "1")},
			})
			h.expect(r, http.StatusCreated, "burst write")
			if i == writes/2 {
				// Stop the relay and the CDC job while events are still flowing.
				first.stop(t)
				cdc.Stop(t)
			}
		}
		cdc.Start(t)
		second := startRelayApp(t, cfg)
		defer second.stop(t)
		// 2 legs, the hold's pending and expired events, and one event per burst write.
		want := 2 + 2 + writes
		records := readEvents(t, broker, want, 2*time.Minute)
		distinct := map[string]int{}
		for _, r := range records {
			distinct[r.tbTimestamp]++
		}
		if len(distinct) != want {
			t.Fatalf("%d distinct events in Kafka, want %d: a gap", len(distinct), want)
		}
		t.Logf("%d records for %d events: %d duplicates", len(records), want, len(records)-want)
	})
}

func waitFor(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
