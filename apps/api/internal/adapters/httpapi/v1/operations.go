package v1

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/logging"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

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

// StuckRequest is a pending write record the reconciler could not settle.
type StuckRequest struct {
	Ledger    scalar.LedgerName `json:"ledger"`
	Key       string            `json:"key"`
	Operation scalar.Operation  `json:"operation"`
	CreatedAt time.Time         `json:"createdAt"`
	Attempts  int               `json:"attempts"`
	Note      *string           `json:"note,omitempty"`
}

// ReconcilerStatus is the answer of GET /v1/_reconciler.
type ReconcilerStatus struct {
	Running            bool       `json:"running"`
	GracePeriodSeconds int        `json:"gracePeriodSeconds" doc:"A pending record younger than this is a write in progress, not a crash."`
	IntervalSeconds    int        `json:"intervalSeconds"`
	LastRunAt          *time.Time `json:"lastRunAt,omitempty"`
	LastSettled        *int       `json:"lastSettled,omitempty" doc:"Records settled by the last run."`
	Pending            int64      `json:"pending" doc:"Pending records, across all ledgers."`
	Stuck              int64      `json:"stuck" doc:"Pending records older than the grace period that the reconciler could not settle."`
	// StuckRequests is always written, as an empty list when nothing is stuck.
	StuckRequests []StuckRequest `json:"stuckRequests" required:"false"`
}

// body wraps an answer for Huma, which takes the response body from a field named Body.
type body[T any] struct {
	Body T
}

// readinessAnswer carries its status: 200 when every dependency answers, 503 otherwise.
type readinessAnswer struct {
	Status int
	Body   Readiness
}

func registerOperations(api huma.API, h *handlers) {
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

	restapi.Register(api, huma.Operation{
		OperationID: "getReconcilerStatus", Method: http.MethodGet, Path: "/v1/_reconciler", Tags: tags,
		Summary: "Reconciler status",
		Description: "The background loop that settles write records left pending by a crash between Postgres\n" +
			"and TigerBeetle (ADR-0007): its last run and the records it is waiting on.\n",
		Responses: restapi.Success(http.StatusOK, "Reconciler status."),
	}, h.getReconcilerStatus)
}

// getLiveness answers as long as the process serves HTTP. It deliberately checks nothing else: a
// liveness failure makes the orchestrator restart the process, which does not help when a
// dependency is down.
func (*handlers) getLiveness(context.Context, *struct{}) (*body[Liveness], error) {
	return &body[Liveness]{Body: Liveness{Status: "ok"}}, nil
}

// getReadiness probes both dependencies and answers 503 when any is down.
func (h *handlers) getReadiness(ctx context.Context, _ *struct{}) (*readinessAnswer, error) {
	report := h.Status.Check(ctx)
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
func (h *handlers) getInfo(ctx context.Context, _ *struct{}) (*body[Info], error) {
	report := h.Status.Check(ctx)
	logReport(ctx, report)
	rt := Runtime{
		StartedAt:                h.Service.StartedAt,
		GoVersion:                runtime.Version(),
		TigerbeetleClientVersion: h.Service.TigerBeetleClientVersion,
		TigerbeetleClusterID:     h.Service.TigerBeetleClusterID,
	}
	if report.Postgres.Up() {
		version, schema := h.Status.Database(ctx)
		rt.PostgresVersion = optional(version)
		if schema > 0 {
			rt.SchemaVersion = &schema
		}
	}
	return &body[Info]{Body: Info{
		Service:      h.Service.Name,
		Version:      h.Service.Version,
		Dependencies: dependenciesOf(report),
		Runtime:      &rt,
	}}, nil
}

// getReconcilerStatus reports the reconciler's last run and the records it waits on.
func (h *handlers) getReconcilerStatus(ctx context.Context, _ *struct{}) (*body[ReconcilerStatus], error) {
	st, err := h.Reconciler.Status(ctx)
	if err != nil {
		return nil, err
	}
	out := ReconcilerStatus{
		Running:            st.Running,
		GracePeriodSeconds: int(st.Grace / time.Second),
		IntervalSeconds:    int(st.Interval / time.Second),
		Pending:            st.Pending,
		Stuck:              st.Stuck,
		StuckRequests:      make([]StuckRequest, 0, len(st.Records)),
	}
	if !st.LastRunAt.IsZero() {
		at := st.LastRunAt.UTC()
		settled := st.LastSettled
		out.LastRunAt, out.LastSettled = &at, &settled
	}
	for _, r := range st.Records {
		note := r.Note
		out.StuckRequests = append(out.StuckRequests, StuckRequest{
			Ledger: scalar.LedgerName(r.Ledger), Key: r.Key, Operation: scalar.Operation(r.Operation),
			CreatedAt: r.CreatedAt, Attempts: r.Attempts, Note: &note,
		})
	}
	return &body[ReconcilerStatus]{Body: out}, nil
}

func dependenciesOf(report app.Report) Dependencies {
	return Dependencies{Postgres: checkOf(report.Postgres), Tigerbeetle: checkOf(report.TigerBeetle)}
}

func checkOf(p app.Probe) DependencyCheck {
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
func logReport(ctx context.Context, report app.Report) {
	logger := logging.From(ctx)
	if err := report.Postgres.Err; err != nil {
		logger.WarnContext(ctx, "postgres probe failed", slog.Any("error", err))
	}
	if err := report.TigerBeetle.Err; err != nil {
		logger.WarnContext(ctx, "tigerbeetle probe failed", slog.Any("error", err))
	}
}

// optional returns nil for "", so an absent value is left out of the JSON.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
