package graphql

import (
	accessapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	accessgql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/graphql"
	bookkeepingapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	bookkeepinggql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/graphql"
	healthdto "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/dto"
	healthgql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/graphql"
	ledgersapp "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	ledgersgql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/graphql"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/transport/graphql/exec"
)

// Modules holds what each module's resolvers call.
type Modules struct {
	Access      accessapp.Application
	Health      healthdto.Source
	Ledgers     ledgersapp.Application
	Bookkeeping bookkeepingapp.Application
}

// Resolver is the gqlgen root. Each module contributes its resolvers by embedding (plan section
// 8.9).
//
// Go note: embedding a pointer to a struct promotes its methods. If two modules defined the same
// method, the selector would be ambiguous and the root would no longer satisfy gqlgen's interface,
// a compile error; a method a module forgets is the same compile error, naming it.
type Resolver struct {
	query    *queryResolver
	mutation *mutationResolver
	ledger   *ledgerResolver
}

// Every module names its resolvers Queries and Mutations; these aliases give each embedded field
// its own name, as Go names an embedded field after its type.
type (
	accessQueries        = accessgql.Queries
	accessMutations      = accessgql.Mutations
	healthQueries        = healthgql.Queries
	ledgersQueries       = ledgersgql.Queries
	ledgersMutations     = ledgersgql.Mutations
	bookkeepingMutations = bookkeepinggql.Mutations
)

type queryResolver struct {
	*accessQueries
	*healthQueries
	*ledgersQueries
}

type mutationResolver struct {
	*accessMutations
	*ledgersMutations
	*bookkeepingMutations
}

var _ exec.ResolverRoot = (*Resolver)(nil)

// NewResolver builds the root from the modules' use cases.
func NewResolver(m Modules) *Resolver {
	return &Resolver{
		query: &queryResolver{
			accessQueries:  accessgql.NewQueries(m.Access),
			healthQueries:  healthgql.NewQueries(m.Health),
			ledgersQueries: ledgersgql.NewQueries(m.Ledgers),
		},
		mutation: &mutationResolver{
			accessMutations:      accessgql.NewMutations(m.Access),
			ledgersMutations:     ledgersgql.NewMutations(m.Ledgers),
			bookkeepingMutations: bookkeepinggql.NewMutations(m.Bookkeeping),
		},
		ledger: &ledgerResolver{
			LedgerFields: ledgersgql.NewLedgerFields(m.Ledgers),
			bookkeeping:  bookkeepinggql.NewLedgerFields(m.Bookkeeping),
		},
	}
}

// Query returns the query resolvers.
func (r *Resolver) Query() exec.QueryResolver { return r.query }

// Mutation returns the mutation resolvers.
func (r *Resolver) Mutation() exec.MutationResolver { return r.mutation }

// Ledger returns the resolvers of the Ledger fields.
func (r *Resolver) Ledger() exec.LedgerResolver { return r.ledger }
