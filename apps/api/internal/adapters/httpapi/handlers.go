package httpapi

import (
	"context"
	"log/slog"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/httpapi/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
)

// server implements gen.StrictServerInterface. Each method is one operation from the spec.
type server struct {
	status  *app.Status
	service string
	version string
}

var _ gen.StrictServerInterface = (*server)(nil)

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

// GetInfo reports the service version and dependency reachability. Unlike readiness it always
// answers 200: the dependency state is the payload, not the health of this endpoint.
func (s *server) GetInfo(ctx context.Context, _ gen.GetInfoRequestObject) (gen.GetInfoResponseObject, error) {
	report := s.status.Check(ctx)
	logReport(ctx, report)
	return gen.GetInfo200JSONResponse{
		Service:      s.service,
		Version:      s.version,
		Dependencies: toDependencies(report),
	}, nil
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
