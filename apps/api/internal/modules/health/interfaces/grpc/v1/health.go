// Package v1 is the health module's gRPC interface (ADR-0021): the standard grpc.health.v1.Health
// service for liveness and readiness, and OperationsService.GetInfo.
package v1

import (
	"context"

	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

// HealthServer answers grpc.health.v1.Health. A Check of the whole server ("" service) probes the
// dependencies on demand, as GET /readyz does: SERVING when Postgres and TigerBeetle answer,
// NOT_SERVING otherwise (plan section 13.2). Watch and List come from grpc-go's health server,
// whose status each Check refreshes.
type HealthServer struct {
	*health.Server
	source dto.Source
}

// NewHealthServer builds the health service.
func NewHealthServer(source dto.Source) *HealthServer {
	return &HealthServer{Server: health.NewServer(), source: source}
}

// Check reports whether the server can serve. A named service is answered by the embedded
// server, which knows none, so it is NOT_FOUND.
func (s *HealthServer) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	if req.GetService() != "" {
		return s.Server.Check(ctx, req)
	}
	st := healthpb.HealthCheckResponse_NOT_SERVING
	if _, ready := s.source.ReadinessNow(ctx); ready {
		st = healthpb.HealthCheckResponse_SERVING
	}
	s.SetServingStatus("", st)
	return &healthpb.HealthCheckResponse{Status: st}, nil
}

// InfoServer implements OperationsService.GetInfo. internal/transport/grpc joins it with the
// bookkeeping module's GetReconcilerStatus.
type InfoServer struct {
	source dto.Source
}

// NewInfoServer builds the info method.
func NewInfoServer(source dto.Source) *InfoServer {
	return &InfoServer{source: source}
}

// GetInfo reads the service, its dependencies and its runtime.
func (s *InfoServer) GetInfo(ctx context.Context, _ *pb.GetInfoRequest) (*pb.GetInfoResponse, error) {
	info := s.source.InfoNow(ctx)
	out := &pb.GetInfoResponse{
		Service: info.Service,
		Version: info.Version,
		Dependencies: map[string]*pb.DependencyCheck{
			dto.ProbePostgres:    checkOf(info.Dependencies.Postgres),
			dto.ProbeTigerBeetle: checkOf(info.Dependencies.Tigerbeetle),
		},
	}
	if mq := info.Dependencies.RabbitMQ; mq != nil {
		out.Dependencies[dto.ProbeRabbitMQ] = checkOf(*mq)
	}
	if nats := info.Dependencies.NATS; nats != nil {
		out.Dependencies[dto.ProbeNATS] = checkOf(*nats)
	}
	if kafka := info.Dependencies.Kafka; kafka != nil {
		out.Dependencies[dto.ProbeKafka] = checkOf(*kafka)
	}
	if kc := info.Dependencies.Keycloak; kc != nil {
		out.Dependencies[dto.ProbeKeycloak] = checkOf(*kc)
	}
	if rt := info.Runtime; rt != nil {
		out.Runtime = &pb.Runtime{
			StartedAt:                grpcapi.Timestamp(rt.StartedAt),
			GoVersion:                rt.GoVersion,
			TigerbeetleClientVersion: rt.TigerbeetleClientVersion,
			TigerbeetleClusterId:     rt.TigerbeetleClusterID,
			PostgresVersion:          rt.PostgresVersion,
			SchemaVersion:            rt.SchemaVersion,
		}
	}
	return out, nil
}

func checkOf(c dto.DependencyCheck) *pb.DependencyCheck {
	return &pb.DependencyCheck{
		Status:    grpcapi.EnumOf[pb.DependencyStatus](string(c.Status)),
		LatencyMs: c.LatencyMs,
		Error:     c.Error,
	}
}
