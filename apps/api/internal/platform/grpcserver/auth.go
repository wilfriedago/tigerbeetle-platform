package grpcserver

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Authenticator checks the credential of a call and resolves the organization it names
// (internal/platform/authn).
type Authenticator interface {
	Authenticate(ctx context.Context, authorization, organization string) (context.Context, error)
}

// Metadata keys of the credential and the organization (ADR-0032): the gRPC spellings of the
// Authorization and X-Organization headers.
const (
	AuthorizationKey = "authorization"
	OrganizationKey  = "x-organization"
)

// publicMethods need no credential: the standard health service, which load balancers call, and
// server reflection, which only describes the contract that is public in the repository.
var publicMethods = []string{"/grpc.health.v1.Health/", "/grpc.reflection."}

func isPublic(fullMethod string) bool {
	for _, prefix := range publicMethods {
		if strings.HasPrefix(fullMethod, prefix) {
			return true
		}
	}
	return false
}

// authenticate reads the credential and the organization from the incoming metadata.
func authenticate(ctx context.Context, auth Authenticator) (context.Context, error) {
	var authorization, organization string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get(AuthorizationKey); len(v) > 0 {
			authorization = v[0]
		}
		if v := md.Get(OrganizationKey); len(v) > 0 {
			organization = v[0]
		}
	}
	return auth.Authenticate(ctx, authorization, organization)
}

func unaryAuth(auth Authenticator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isPublic(info.FullMethod) {
			return handler(ctx, req)
		}
		ctx, err := authenticate(ctx, auth)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func streamAuth(auth Authenticator) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isPublic(info.FullMethod) {
			return handler(srv, ss)
		}
		ctx, err := authenticate(ss.Context(), auth)
		if err != nil {
			return err
		}
		return handler(srv, &contextStream{ServerStream: ss, ctx: ctx})
	}
}
