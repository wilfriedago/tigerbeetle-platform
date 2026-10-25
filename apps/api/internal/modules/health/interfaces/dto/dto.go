// Package dto holds the wire types of the health module that every transport shares, and Source,
// which fills them from the platform's readiness probes and build information. The health module
// has no use case of its own (plan section 4.2).
package dto

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/readiness"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// The probe names the readiness checker reports, as bootstrap registers them.
const (
	ProbePostgres    = "postgres"
	ProbeTigerBeetle = "tigerbeetle"
	ProbeRabbitMQ    = "rabbitmq"
	ProbeNATS        = "nats"
	ProbeKafka       = "kafka"
)

// DatabaseInfo reports the Postgres server and schema versions for GET /v1/_info.
type DatabaseInfo interface {
	ServerVersion(ctx context.Context) (string, error)
	SchemaVersion(ctx context.Context) (int64, error)
}

// Service describes the running binary for GET /v1/_info.
type Service struct {
	Name                     string
	Version                  string
	StartedAt                time.Time
	TigerBeetleClientVersion string
	TigerBeetleClusterID     string
}

// Liveness is the answer of GET /healthz.
type Liveness struct {
	Status string `json:"status" const:"ok"`
}

// DependencyCheck is the result of probing one dependency.
type DependencyCheck struct {
	Status    scalar.DependencyStatus `json:"status"`
	LatencyMs int64                   `json:"latencyMs" minimum:"0" doc:"Round-trip time of the probe in milliseconds."`
	Error     *string                 `json:"error,omitempty" doc:"A short, client-safe reason when the status is down. Never raw driver output."`
}

// TransformSchema adds the description of DependencyCheck; Huma has no struct tag for it.
func (DependencyCheck) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Description = "The result of probing one dependency."
	return s
}

// Dependencies reports each dependency the service needs. The brokers appear only when a
// transport that uses them is enabled.
type Dependencies struct {
	Postgres    DependencyCheck  `json:"postgres"`
	Tigerbeetle DependencyCheck  `json:"tigerbeetle"`
	RabbitMQ    *DependencyCheck `json:"rabbitmq,omitempty" doc:"Present when AMQP RPC or the Kafka ledger event relay is enabled."`
	NATS        *DependencyCheck `json:"nats,omitempty" doc:"Present when NATS RPC is enabled."`
	Kafka       *DependencyCheck `json:"kafka,omitempty" doc:"Present when the Kafka ledger event relay is enabled."`
}

// Readiness is the answer of GET /readyz. degraded means Postgres and TigerBeetle answer but a
// broker does not: the service still serves HTTP, so the answer stays 200.
type Readiness struct {
	Status       string       `json:"status" enum:"ready,not_ready,degraded"`
	Dependencies Dependencies `json:"dependencies"`
}

// Runtime describes the running service.
type Runtime struct {
	StartedAt                time.Time `json:"startedAt"`
	GoVersion                string    `json:"goVersion" example:"go1.27.3"`
	TigerbeetleClientVersion string    `json:"tigerbeetleClientVersion" example:"v0.17.10"`
	TigerbeetleClusterID     string    `json:"tigerbeetleClusterId" doc:"The cluster ID, a u128 in decimal."`
	PostgresVersion          *string   `json:"postgresVersion,omitempty" doc:"The server version reported by Postgres, when reachable."`
	SchemaVersion            *int64    `json:"schemaVersion,omitempty" doc:"The last applied migration, when reachable."`
}

// TransformSchema adds the description of Runtime.
func (Runtime) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Description = "Versions and settings of the running service."
	return s
}

// Info is the answer of GET /v1/_info.
type Info struct {
	Service      string       `json:"service" example:"tigerbeetle-platform-api"`
	Version      string       `json:"version" example:"0.0.1"`
	Dependencies Dependencies `json:"dependencies"`
	Runtime      *Runtime     `json:"runtime,omitempty"`
}

// TransformSchema describes the version field, whose text holds backticks a struct tag cannot.
func (Info) TransformSchema(_ huma.Registry, s *huma.Schema) *huma.Schema {
	s.Properties["version"].Description = "Build version injected at compile time, or `dev` for local builds."
	return s
}

// Source is what the health answers read: the readiness probes, the database versions and the
// running binary. cmd/openapi and cmd/graphqlschema pass the zero value, because registering an
// operation never reads it.
type Source struct {
	Readiness *readiness.Checker
	Database  DatabaseInfo
	Service   Service
}

// ReadinessNow probes the dependencies and reports whether every required one answered.
func (s Source) ReadinessNow(ctx context.Context) (Readiness, bool) {
	report := s.Readiness.Check(ctx)
	logReport(ctx, report)
	out := Readiness{Status: "ready", Dependencies: dependenciesOf(report)}
	switch {
	case !report.Ready():
		out.Status = "not_ready"
	case report.Degraded():
		out.Status = "degraded"
	}
	return out, report.Ready()
}

// InfoNow reports the service version, runtime versions and dependency reachability.
func (s Source) InfoNow(ctx context.Context) Info {
	report := s.Readiness.Check(ctx)
	logReport(ctx, report)
	rt := Runtime{
		StartedAt:                s.Service.StartedAt,
		GoVersion:                runtime.Version(),
		TigerbeetleClientVersion: s.Service.TigerBeetleClientVersion,
		TigerbeetleClusterID:     s.Service.TigerBeetleClusterID,
	}
	if pg, ok := report.Result(ProbePostgres); ok && pg.Up() && s.Database != nil {
		version, schema := s.database(ctx)
		rt.PostgresVersion = restapi.Optional(version)
		if schema > 0 {
			rt.SchemaVersion = &schema
		}
	}
	return Info{Service: s.Service.Name, Version: s.Service.Version, Dependencies: dependenciesOf(report), Runtime: &rt}
}

// database returns the Postgres server version and the last applied migration, or zero values
// when Postgres does not tell within the probe timeout.
func (s Source) database(ctx context.Context) (version string, schema int64) {
	ctx, cancel := context.WithTimeout(ctx, s.Readiness.Timeout())
	defer cancel()
	version, _ = s.Database.ServerVersion(ctx)
	schema, _ = s.Database.SchemaVersion(ctx)
	return version, schema
}

func dependenciesOf(report readiness.Report) Dependencies {
	pg, _ := report.Result(ProbePostgres)
	tb, _ := report.Result(ProbeTigerBeetle)
	out := Dependencies{Postgres: checkOf(pg), Tigerbeetle: checkOf(tb)}
	if mq, ok := report.Result(ProbeRabbitMQ); ok {
		check := checkOf(mq)
		out.RabbitMQ = &check
	}
	if nats, ok := report.Result(ProbeNATS); ok {
		check := checkOf(nats)
		out.NATS = &check
	}
	if kafka, ok := report.Result(ProbeKafka); ok {
		check := checkOf(kafka)
		out.Kafka = &check
	}
	return out
}

func checkOf(p readiness.Result) DependencyCheck {
	check := DependencyCheck{Status: scalar.DependencyUp, LatencyMs: p.Latency.Milliseconds()}
	if !p.Up() {
		reason := dependencyReason(p.Err)
		check.Status = scalar.DependencyDown
		check.Error = &reason
	}
	return check
}

// dependencyReason turns a probe error into the short, client-safe reason shown in readiness
// answers. The full error goes to the log only.
func dependencyReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "unreachable"
}

// logReport records failed probes with their full error, which clients never see.
func logReport(ctx context.Context, report readiness.Report) {
	logger := logging.From(ctx)
	for _, r := range report {
		if r.Err != nil {
			logger.WarnContext(ctx, r.Name+" probe failed", slog.Any("error", r.Err))
		}
	}
}
