// Package v1 is version 1 of the health module's REST interface: liveness, readiness and service
// information (plan section 4.2). The health module has no use case of its own; package dto reads
// the platform's readiness probes and build information.
package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// Deps is what the health operations read.
type Deps = dto.Source

// notReady holds the extension members of the 503 problem of /readyz.
type notReady struct {
	Dependencies dto.Dependencies `json:"dependencies"`
}

type handlers struct {
	source dto.Source
}

// Register adds the health operations to api, under the tag operations.
func Register(api huma.API, deps Deps) {
	h := &handlers{source: deps}
	tags := []string{"operations"}
	restapi.Register(api, huma.Operation{
		OperationID: "getLiveness", Method: http.MethodGet, Path: "/healthz", Tags: tags,
		Summary:     "Liveness probe",
		Description: "Returns 200 as soon as the process serves HTTP. It does not check dependencies.",
		Responses:   restapi.Success(http.StatusOK, "The process is alive."),
	}, h.getLiveness)

	responses := restapi.Success(http.StatusOK, "Every required dependency is reachable.")
	responses["503"] = restapi.ProblemWith[notReady](api,
		"A required dependency is unreachable: a `not-ready` problem whose `dependencies` reports each one.")
	restapi.Register(api, huma.Operation{
		OperationID: "getReadiness", Method: http.MethodGet, Path: "/readyz", Tags: tags,
		Summary: "Readiness probe",
		Description: "Checks Postgres and TigerBeetle with a short timeout and reports each one. Returns a\n" +
			"503 `not-ready` problem when any required dependency is down, so load balancers stop routing\n" +
			"traffic to this instance.\n",
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
func (*handlers) getLiveness(context.Context, *struct{}) (*restapi.Body[dto.Liveness], error) {
	return &restapi.Body[dto.Liveness]{Body: dto.Liveness{Status: "ok"}}, nil
}

// getReadiness probes the dependencies and answers a 503 problem when a required one is down. The
// problem is the contract of the operation, not a failure of it, so it is not logged as one.
func (h *handlers) getReadiness(ctx context.Context, _ *struct{}) (*restapi.Body[dto.Readiness], error) {
	ready, ok := h.source.ReadinessNow(ctx)
	if !ok {
		e := restapi.NewError(ctx, problem.From(ledger.NewProblem(ledger.ErrUnavailable, "not-ready",
			"a required dependency is unreachable; dependencies reports each one")))
		e.Extensions = map[string]any{"dependencies": ready.Dependencies}
		return nil, e
	}
	return &restapi.Body[dto.Readiness]{Body: ready}, nil
}

// getInfo reports the service version, runtime versions and dependency reachability. Unlike
// readiness it always answers 200: the dependency state is the payload, not the health of this
// endpoint.
func (h *handlers) getInfo(ctx context.Context, _ *struct{}) (*restapi.Body[dto.Info], error) {
	return &restapi.Body[dto.Info]{Body: h.source.InfoNow(ctx)}, nil
}
