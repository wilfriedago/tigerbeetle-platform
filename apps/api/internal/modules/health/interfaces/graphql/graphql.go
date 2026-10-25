// Package graphql is the health module's GraphQL interface: Query.serviceInfo. The registration
// imports neither gqlgen nor the generated executor (plan section 8.7); the resolver has the
// signature gqlgen generates from it.
package graphql

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/graphqlschema"
)

// Register adds the health fields to the schema.
func Register(s *graphqlschema.Schema) {
	graphqlschema.Describe[dto.Info](s, "The running service: its version, its runtime and the reachability of its dependencies.")
	graphqlschema.Describe[dto.Dependencies](s, "Each dependency the service needs.")
	graphqlschema.Describe[dto.DependencyCheck](s, "The result of probing one dependency.")
	graphqlschema.Describe[dto.Runtime](s, "Versions and settings of the running service.")
	graphqlschema.Query[graphqlschema.NoArgs, dto.Info](s, "serviceInfo",
		"Service name and version, runtime versions, and the reachability of Postgres and TigerBeetle.")
}

// Queries holds the health query resolvers. Resolver types are not named after schema types, so
// gqlgen's autobind never mistakes them for one.
type Queries struct {
	source dto.Source
}

// NewQueries builds the resolvers on the platform's probes and build information.
func NewQueries(source dto.Source) *Queries {
	return &Queries{source: source}
}

// ServiceInfo resolves Query.serviceInfo.
func (q *Queries) ServiceInfo(ctx context.Context) (*dto.Info, error) {
	info := q.source.InfoNow(ctx)
	return &info, nil
}
