package main

import (
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcserver"
)

// The ErrorInfo domain of gRPC errors is the service name of GET /v1/_info (ADR-0021).
func TestErrorDomainIsTheServiceName(t *testing.T) {
	t.Parallel()
	if serviceName != grpcserver.ErrorDomain {
		t.Fatalf("service %q, ErrorInfo domain %q", serviceName, grpcserver.ErrorDomain)
	}
}
