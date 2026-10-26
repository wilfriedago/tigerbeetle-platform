// Package schema collects the GraphQL registrations of every module into one schema (plan section
// 8.7). It never imports gqlgen or the generated executor, so cmd/graphqlschema compiles even when
// the executor is stale and `just generate` can rebuild it.
package schema

import (
	accessgql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/graphql"
	bookkeepinggql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/graphql"
	healthgql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/health/interfaces/graphql"
	ledgersgql "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/graphql"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/graphqlschema"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// RegisterAll adds the shared scalars and enums, then every module's fields.
func RegisterAll(s *graphqlschema.Schema) {
	graphqlschema.Scalar[scalar.Amount](s, "Amount", "Unsigned 128-bit integer in the asset's smallest unit, as a decimal string.")
	graphqlschema.Scalar[scalar.SignedAmount](s, "SignedAmount", "Signed balance in the asset's smallest unit, as a decimal string.")
	graphqlschema.Scalar[scalar.ID](s, "U128", "TigerBeetle 128-bit ID: 32 lowercase hexadecimal characters, big-endian, time ordered.")
	graphqlschema.Scalar[scalar.Timestamp](s, "U64", "TigerBeetle timestamp: nanoseconds since the Unix epoch, as a decimal string.")
	graphqlschema.Scalar[scalar.Metadata](s, "Metadata", "Key/value pairs for your own systems: a JSON object of strings, at most 64 entries.")
	graphqlschema.Scalar[scalar.JSON](s, "JSON", "A free-form JSON object: the stored request and response of a write.")
	graphqlschema.Describe[scalar.BalanceLimit](s, "The balance limit flag of the TigerBeetle accounts a rule creates.")
	graphqlschema.Describe[scalar.HoldStatus](s, "The life cycle of a hold. expired is derived from the cluster clock.")
	graphqlschema.Describe[scalar.Operation](s, "The kind of write an idempotency record holds.")
	graphqlschema.Describe[scalar.RequestStatus](s, "The state of an idempotency record.")
	graphqlschema.Describe[scalar.DependencyStatus](s, "Whether a dependency answered its probe.")

	healthgql.Register(s)
	accessgql.Register(s)
	ledgersgql.Register(s)
	bookkeepinggql.Register(s)
}
