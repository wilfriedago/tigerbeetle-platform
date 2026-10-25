package httpapi

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
)

// server implements gen.StrictServerInterface. Each method is one operation from the spec; it
// decodes the request, calls one use case and maps the result. Errors go back to the strict
// handler, which renders them as problems (problem.go).
type server struct {
	status       *app.Status
	ledgers      *app.Ledgers
	settings     *app.Settings
	accounts     *app.Accounts
	transactions *app.Transactions
	holds        *app.Holds
	overview     *app.Overview
	reconciler   *app.Reconciler
	service      string
	version      string
	runtime      Runtime
}

var _ gen.StrictServerInterface = (*server)(nil)

// Runtime describes the running binary for GET /v1/_info.
type Runtime struct {
	StartedAt                time.Time
	TigerBeetleClientVersion string
	TigerBeetleClusterID     string
}

// GetLiveness answers as long as the process serves HTTP. It deliberately checks nothing else:
// a liveness failure makes the orchestrator restart the process, which does not help when a
// dependency is down.
func (s *server) GetLiveness(context.Context, gen.GetLivenessRequestObject) (gen.GetLivenessResponseObject, error) {
	return gen.GetLiveness200JSONResponse{Status: gen.Ok}, nil
}

// GetReadiness probes both dependencies and answers 503 when any is down.
func (s *server) GetReadiness(ctx context.Context, _ gen.GetReadinessRequestObject) (gen.GetReadinessResponseObject, error) {
	report := s.status.Check(ctx)
	logReport(ctx, report)
	body := gen.Readiness{Status: gen.Ready, Dependencies: toDependencies(report)}
	if !report.Ready() {
		body.Status = gen.NotReady
		return gen.GetReadiness503JSONResponse(body), nil
	}
	return gen.GetReadiness200JSONResponse(body), nil
}

// GetInfo reports the service version, runtime versions and dependency reachability. Unlike
// readiness it always answers 200: the dependency state is the payload, not the health of this
// endpoint.
func (s *server) GetInfo(ctx context.Context, _ gen.GetInfoRequestObject) (gen.GetInfoResponseObject, error) {
	report := s.status.Check(ctx)
	logReport(ctx, report)
	rt := gen.Runtime{
		StartedAt:                s.runtime.StartedAt,
		GoVersion:                runtime.Version(),
		TigerbeetleClientVersion: s.runtime.TigerBeetleClientVersion,
		TigerbeetleClusterId:     s.runtime.TigerBeetleClusterID,
	}
	if report.Postgres.Up() {
		version, schema := s.status.Database(ctx)
		rt.PostgresVersion = optional(version)
		if schema > 0 {
			rt.SchemaVersion = &schema
		}
	}
	return gen.GetInfo200JSONResponse{
		Service:      s.service,
		Version:      s.version,
		Dependencies: toDependencies(report),
		Runtime:      &rt,
	}, nil
}

// GetReconcilerStatus reports the reconciler's last run and the records it waits on.
func (s *server) GetReconcilerStatus(ctx context.Context, _ gen.GetReconcilerStatusRequestObject) (gen.GetReconcilerStatusResponseObject, error) {
	st, err := s.reconciler.Status(ctx)
	if err != nil {
		return nil, err
	}
	body := gen.ReconcilerStatus{
		Running:            st.Running,
		GracePeriodSeconds: int(st.Grace / time.Second),
		IntervalSeconds:    int(st.Interval / time.Second),
		Pending:            st.Pending,
		Stuck:              st.Stuck,
	}
	if !st.LastRunAt.IsZero() {
		at := st.LastRunAt.UTC()
		settled := st.LastSettled
		body.LastRunAt, body.LastSettled = &at, &settled
	}
	stuck := make([]gen.StuckRequest, 0, len(st.Records))
	for _, r := range st.Records {
		note := r.Note
		stuck = append(stuck, gen.StuckRequest{Ledger: r.Ledger, Key: r.Key, Operation: gen.Operation(r.Operation), CreatedAt: r.CreatedAt, Attempts: r.Attempts, Note: &note})
	}
	body.StuckRequests = &stuck
	return gen.GetReconcilerStatus200JSONResponse(body), nil
}

func toDependencies(report app.Report) gen.Dependencies {
	return gen.Dependencies{
		Postgres:    toCheck(report.Postgres),
		Tigerbeetle: toCheck(report.TigerBeetle),
	}
}

func toCheck(p app.Probe) gen.DependencyCheck {
	check := gen.DependencyCheck{Status: gen.Up, LatencyMs: p.Latency.Milliseconds()}
	if !p.Up() {
		reason := dependencyReason(p.Err)
		check.Status = gen.Down
		check.Error = &reason
	}
	return check
}

// logReport records failed probes with their full error, which clients never see.
func logReport(ctx context.Context, report app.Report) {
	logger := loggerFrom(ctx)
	if err := report.Postgres.Err; err != nil {
		logger.WarnContext(ctx, "postgres probe failed", slog.Any("error", err))
	}
	if err := report.TigerBeetle.Err; err != nil {
		logger.WarnContext(ctx, "tigerbeetle probe failed", slog.Any("error", err))
	}
}
