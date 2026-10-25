// Package v1 is the health module's broker RPC interface: liveness, readiness and service
// information, as v1.getLiveness, v1.getReadiness and v1.getInfo. Readiness answers ok with its
// status, ready or not; the reply is about the call, not the service.
package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/rpc"
)

// Register adds the health methods.
func Register(r *rpc.Router, source dto.Source) {
	r.Handle("v1.getLiveness", rpc.Bind(func(context.Context, struct{}, rpc.NoBody, rpc.Request) (any, error) {
		return dto.Liveness{Status: "ok"}, nil
	}))
	r.Handle("v1.getReadiness", rpc.Bind(func(ctx context.Context, _ struct{}, _ rpc.NoBody, _ rpc.Request) (any, error) {
		readiness, _ := source.ReadinessNow(ctx)
		return readiness, nil
	}))
	r.Handle("v1.getInfo", rpc.Bind(func(ctx context.Context, _ struct{}, _ rpc.NoBody, _ rpc.Request) (any, error) {
		return source.InfoNow(ctx), nil
	}))
}
