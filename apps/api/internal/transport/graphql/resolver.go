package graphql

import (
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	healthgql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/graphql"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/graphql/exec"
)

// Modules holds what each module's resolvers call.
type Modules struct {
	Health healthdto.Source
}

// Resolver is the gqlgen root. Each module contributes its resolvers by embedding (plan section
// 8.9).
//
// Go note: embedding a pointer to a struct promotes its methods. If two modules defined the same
// method, the selector would be ambiguous and the root would no longer satisfy gqlgen's interface,
// a compile error; a method a module forgets is the same compile error, naming it.
type Resolver struct {
	query *queryResolver
}

type queryResolver struct {
	*healthgql.Queries
}

var _ exec.ResolverRoot = (*Resolver)(nil)

// NewResolver builds the root from the modules' use cases.
func NewResolver(m Modules) *Resolver {
	return &Resolver{query: &queryResolver{Queries: healthgql.NewQueries(m.Health)}}
}

// Query returns the query resolvers.
func (r *Resolver) Query() exec.QueryResolver { return r.query }
