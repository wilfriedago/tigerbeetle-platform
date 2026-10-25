package app

import (
	"context"
	"sync"
	"time"
)

// Probe is the outcome of checking one dependency.
type Probe struct {
	// Err is nil when the dependency answered in time. It may wrap context.DeadlineExceeded.
	Err     error
	Latency time.Duration
}

// Up reports whether the dependency answered.
func (p Probe) Up() bool { return p.Err == nil }

// Report is the readiness of every dependency.
type Report struct {
	Postgres    Probe
	TigerBeetle Probe
}

// Ready is true only when every dependency is up.
func (r Report) Ready() bool { return r.Postgres.Up() && r.TigerBeetle.Up() }

// Prober answers a readiness probe. Both store adapters implement it.
type Prober interface {
	Ready(ctx context.Context) error
}

// DatabaseInfo reports the Postgres server and schema versions, for the System page.
type DatabaseInfo interface {
	ServerVersion(ctx context.Context) (string, error)
	SchemaVersion(ctx context.Context) (int64, error)
}

// Status answers readiness and service-information requests by probing the stores.
type Status struct {
	ledgerStore   Prober
	metadataStore Prober
	timeout       time.Duration
}

// NewStatus builds the use case. timeout bounds each probe independently.
func NewStatus(ledgerStore, metadataStore Prober, timeout time.Duration) *Status {
	return &Status{ledgerStore: ledgerStore, metadataStore: metadataStore, timeout: timeout}
}

// Database returns the Postgres server version and the last applied migration, or zero values
// when the store cannot tell or does not answer within the probe timeout.
func (s *Status) Database(ctx context.Context) (version string, schema int64) {
	info, ok := s.metadataStore.(DatabaseInfo)
	if !ok {
		return "", 0
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	version, _ = info.ServerVersion(ctx)
	schema, _ = info.SchemaVersion(ctx)
	return version, schema
}

// Check probes both dependencies concurrently, each with its own timeout, so a slow dependency
// does not hide the state of the other.
func (s *Status) Check(ctx context.Context) Report {
	var (
		report Report
		wg     sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		report.Postgres = s.probe(ctx, s.metadataStore.Ready)
	}()
	go func() {
		defer wg.Done()
		report.TigerBeetle = s.probe(ctx, s.ledgerStore.Ready)
	}()
	wg.Wait()
	return report
}

func (s *Status) probe(ctx context.Context, ready func(context.Context) error) Probe {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	start := time.Now()
	err := ready(ctx)
	return Probe{Err: err, Latency: time.Since(start)}
}
