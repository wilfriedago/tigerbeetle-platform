// Package v1 is version 1 of the health module's REST interface: liveness, readiness and service
// information (plan section 4.2). The health module has no use case of its own; it reads the
// platform's readiness probes and build information.
package v1

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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
)

// Deps is what the health operations read. cmd/openapi passes the zero value, because
// registering an operation never calls it.
type Deps struct {
	Readiness *readiness.Checker
	Database  DatabaseInfo
	Service   Service
}

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

// Dependencies reports each dependency the service needs.
type Dependencies struct {
	Postgres    DependencyCheck `json:"postgres"`
	Tigerbeetle DependencyCheck `json:"tigerbeetle"`
}

// Readiness is the answer of GET /readyz.
type Readiness struct {
	Status       string       `json:"status" enum:"ready,not_ready"`
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

// readinessAnswer carries its status: 200 when every dependency answers, 503 otherwise.
type readinessAnswer struct {
	Status int
	Body   Readiness
}

type handlers struct {
	Deps
}

// Register adds the health operations to api, under the tag operations.
func Register(api huma.API, deps Deps) {
	h := &handlers{Deps: deps}
	tags := []string{"operations"}
	restapi.Register(api, huma.Operation{
		OperationID: "getLiveness", Method: http.MethodGet, Path: "/healthz", Tags: tags,
		Summary:     "Liveness probe",
		Description: "Returns 200 as soon as the process serves HTTP. It does not check dependencies.",
		Responses:   restapi.Success(http.StatusOK, "The process is alive."),
	}, h.getLiveness)

	responses := restapi.Success(http.StatusOK, "Every dependency is reachable.")
	responses["503"] = restapi.JSON[Readiness](api, "At least one dependency is unreachable.")
	restapi.Register(api, huma.Operation{
		OperationID: "getReadiness", Method: http.MethodGet, Path: "/readyz", Tags: tags,
		Summary: "Readiness probe",
		Description: "Checks Postgres and TigerBeetle with a short timeout and reports each one. Returns 503 when\n" +
			"any dependency is down so load balancers stop routing traffic to this instance.\n",
		Responses: responses,
	}, h.getReadiness)

	restapi.Register(api, huma.Operation{
		OperationID: "getInfo", Method: http.MethodGet, Path: "/v1/_info", Tags: tags,
		Summary:     "Service information",
		Description: "Service name and version, runtime versions, and the reachability of Postgres and TigerBeetle.",
		Responses:   restapi.Success(http.StatusOK, "Service information."),
	}, h.getInfo)
}

// getLiveness answers as long as the process serves HTTP. It deliberately checks nothing else: a
// liveness failure makes the orchestrator restart the process, which does not help when a
// dependency is down.
func (*handlers) getLiveness(context.Context, *struct{}) (*restapi.Body[Liveness], error) {
	return &restapi.Body[Liveness]{Body: Liveness{Status: "ok"}}, nil
}

// getReadiness probes the dependencies and answers 503 when a required one is down.
func (h *handlers) getReadiness(ctx context.Context, _ *struct{}) (*readinessAnswer, error) {
	report := h.Readiness.Check(ctx)
	logReport(ctx, report)
	out := &readinessAnswer{Status: http.StatusOK, Body: Readiness{Status: "ready", Dependencies: dependenciesOf(report)}}
	if !report.Ready() {
		out.Status = http.StatusServiceUnavailable
		out.Body.Status = "not_ready"
	}
	return out, nil
}

// getInfo reports the service version, runtime versions and dependency reachability. Unlike
// readiness it always answers 200: the dependency state is the payload, not the health of this
// endpoint.
func (h *handlers) getInfo(ctx context.Context, _ *struct{}) (*restapi.Body[Info], error) {
	report := h.Readiness.Check(ctx)
	logReport(ctx, report)
	rt := Runtime{
		StartedAt:                h.Service.StartedAt,
		GoVersion:                runtime.Version(),
		TigerbeetleClientVersion: h.Service.TigerBeetleClientVersion,
		TigerbeetleClusterID:     h.Service.TigerBeetleClusterID,
	}
	if pg, ok := report.Result(ProbePostgres); ok && pg.Up() && h.Database != nil {
		version, schema := h.database(ctx)
		rt.PostgresVersion = restapi.Optional(version)
		if schema > 0 {
			rt.SchemaVersion = &schema
		}
	}
	return &restapi.Body[Info]{Body: Info{
		Service:      h.Service.Name,
		Version:      h.Service.Version,
		Dependencies: dependenciesOf(report),
		Runtime:      &rt,
	}}, nil
}

// database returns the Postgres server version and the last applied migration, or zero values
// when Postgres does not tell within the probe timeout.
func (h *handlers) database(ctx context.Context) (version string, schema int64) {
	ctx, cancel := context.WithTimeout(ctx, h.Readiness.Timeout())
	defer cancel()
	version, _ = h.Database.ServerVersion(ctx)
	schema, _ = h.Database.SchemaVersion(ctx)
	return version, schema
}

func dependenciesOf(report readiness.Report) Dependencies {
	pg, _ := report.Result(ProbePostgres)
	tb, _ := report.Result(ProbeTigerBeetle)
	return Dependencies{Postgres: checkOf(pg), Tigerbeetle: checkOf(tb)}
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
