// Package graphql is the ledgers module's GraphQL interface (ADR-0020): its registrations, its
// types and its resolvers. The registrations import neither gqlgen nor the generated executor
// (plan section 8.7); the resolvers have the signatures gqlgen generates from them and call the
// same use cases as REST.
package graphql

import (
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/graphqlschema"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Register adds the ledgers queries, mutations and fields to the schema.
func Register(s *graphqlschema.Schema) {
	graphqlschema.Describe[Ledger](s, "A ledger is a namespace for accounts and transactions. Money never crosses ledgers.")
	graphqlschema.Describe[dto.Rule](s, "An account rule: the flags of the accounts created for the addresses it matches (ADR-0006).")
	graphqlschema.Describe[dto.RuleMatch](s, "The rule that would create the account of an address, and every rule considered.")
	graphqlschema.Describe[dto.EventType](s, "An event type of a ledger, carried in the code of every transfer of its writes.")

	graphqlschema.Query[LedgerArgs, *Ledger](s, "ledger", "A ledger by name, or null when no ledger has that name.")
	graphqlschema.Query[LedgersArgs, LedgerConnection](s, "ledgers", "Every ledger in name order.")

	graphqlschema.Mutation[CreateLedgerArgs, Ledger](s, "createLedger",
		"Creates a ledger. The name is permanent; a ledger cannot be deleted, because TigerBeetle accounts and transfers are permanent.")
	graphqlschema.Mutation[ReplaceLedgerMetadataArgs, Ledger](s, "replaceLedgerMetadata", "Replaces the metadata of a ledger.")
	graphqlschema.Mutation[PutRuleArgs, dto.Rule](s, "putRule",
		"Creates or replaces the rule for a pattern. It applies to accounts created afterwards.")
	graphqlschema.Mutation[DeleteRuleArgs, bool](s, "deleteRule",
		"Deletes a stored rule. Deleting a stored * or world restores the built-in default.")
	graphqlschema.Mutation[CreateEventTypeArgs, dto.EventType](s, "createEventType",
		"Registers an event type. Event types are append-only: a code never changes meaning. Without a code, the next free one is assigned.")

	graphqlschema.Field[graphqlschema.NoArgs, []dto.Rule](s, "Ledger", "rules",
		"The rules that decide the flags of new accounts, most specific first, built-in defaults included.")
	graphqlschema.Field[RuleMatchArgs, dto.RuleMatch](s, "Ledger", "ruleMatch", "Which rule wins for an address.")
	graphqlschema.Field[graphqlschema.NoArgs, []dto.EventType](s, "Ledger", "eventTypes",
		"The event types of the ledger, the built-in default first, with their TigerBeetle code and usage.")
}

// Ledger is the GraphQL form of a ledger. The bookkeeping module adds the fields about money
// (assets, stats, accounts, transactions, holds, requests, search) through the transport, which
// keeps the two modules apart (plan section 8.9).
type Ledger struct {
	Name         scalar.LedgerName `json:"name"`
	Metadata     scalar.Metadata   `json:"metadata"`
	CreatedAt    time.Time         `json:"createdAt"`
	AssetCodes   []scalar.Asset    `json:"assetCodes" doc:"The assets used so far, in canonical form."`
	AccountCount int64             `json:"accountCount" doc:"The number of addresses in the ledger."`
}

// LedgerConnection is a page of ledgers.
type LedgerConnection struct {
	Nodes    []Ledger               `json:"nodes"`
	PageInfo graphqlschema.PageInfo `json:"pageInfo"`
}

// LedgerArgs names a ledger.
type LedgerArgs struct {
	Name scalar.LedgerName `json:"name"`
}

// LedgersArgs pages the ledgers.
type LedgersArgs struct {
	First *int    `json:"first" default:"25" doc:"Page size, 1 to 100."`
	After *string `json:"after" doc:"The endCursor of the previous page."`
}

// RuleMatchArgs names the address whose rule is asked for.
type RuleMatchArgs struct {
	Address scalar.Address `json:"address"`
}

// CreateLedgerInput is the input of createLedger.
type CreateLedgerInput struct {
	Name     scalar.LedgerName `json:"name" doc:"Lowercase letters, digits, dash and underscore, 1 to 63 characters."`
	Metadata *scalar.Metadata  `json:"metadata"`
}

// CreateLedgerArgs follows the GraphQL convention of one input argument per mutation.
type CreateLedgerArgs struct {
	Input CreateLedgerInput `json:"input"`
}

// ReplaceLedgerMetadataInput is the input of replaceLedgerMetadata.
type ReplaceLedgerMetadataInput struct {
	Ledger   scalar.LedgerName `json:"ledger"`
	Metadata scalar.Metadata   `json:"metadata"`
}

// ReplaceLedgerMetadataArgs wraps the input.
type ReplaceLedgerMetadataArgs struct {
	Input ReplaceLedgerMetadataInput `json:"input"`
}

// PutRuleInput is the input of putRule.
type PutRuleInput struct {
	Ledger  scalar.LedgerName   `json:"ledger"`
	Pattern string              `json:"pattern" doc:"*, an address such as world, or a prefix ending with a colon such as users:."`
	Limit   scalar.BalanceLimit `json:"limit"`
	History *bool               `json:"history" default:"false"`
	Code    int                 `json:"code" doc:"The TigerBeetle account code, 1 to 65535."`
}

// PutRuleArgs wraps the input.
type PutRuleArgs struct {
	Input PutRuleInput `json:"input"`
}

// DeleteRuleInput is the input of deleteRule.
type DeleteRuleInput struct {
	Ledger  scalar.LedgerName `json:"ledger"`
	Pattern string            `json:"pattern"`
}

// DeleteRuleArgs wraps the input.
type DeleteRuleArgs struct {
	Input DeleteRuleInput `json:"input"`
}

// CreateEventTypeInput is the input of createEventType.
type CreateEventTypeInput struct {
	Ledger scalar.LedgerName `json:"ledger"`
	Name   string            `json:"name" doc:"Lowercase, starting with a letter, such as card.authorized."`
	Code   *int              `json:"code" doc:"2 to 65535; the next free code when absent."`
}

// CreateEventTypeArgs wraps the input.
type CreateEventTypeArgs struct {
	Input CreateEventTypeInput `json:"input"`
}
