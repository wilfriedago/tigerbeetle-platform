// Package graphql is the bookkeeping module's GraphQL interface (ADR-0020): the mutations that
// move money and the fields it adds to the Ledger type the ledgers module owns. The registrations
// import neither gqlgen nor the generated executor (plan section 8.7). The resolvers call the same
// use cases as REST; a money mutation takes its Idempotency-Key as an input field, because one
// GraphQL request can carry several mutations and a header could not say which one it belongs to
// (plan section 10).
package graphql

import (
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/graphqlschema"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Register adds the bookkeeping mutations and Ledger fields to the schema.
func Register(s *graphqlschema.Schema) {
	graphqlschema.Describe[dto.Transaction](s, "A committed transaction: one linked chain of TigerBeetle transfers.")
	graphqlschema.Describe[dto.PostingRecord](s, "A committed posting with the TigerBeetle transfer behind it.")
	graphqlschema.Describe[dto.Hold](s, "A TigerBeetle pending transfer and, once resolved, the transfer that resolved it (ADR-0010).")
	graphqlschema.Describe[dto.Account](s, "An address with one TigerBeetle account per asset, and its open holds.")
	graphqlschema.Describe[dto.AccountSummary](s, "An address with its balance per asset.")
	graphqlschema.Describe[dto.AssetSummary](s, "An asset of a ledger with its TigerBeetle ledger number and the money issued.")
	graphqlschema.Describe[dto.LedgerStats](s, "The figures of the ledger overview.")
	graphqlschema.Describe[dto.WriteRequest](s, "An idempotency record: a write with its key, operation and outcome (ADR-0007).")
	graphqlschema.Describe[dto.WriteRequestDetail](s, "A write request with its stored request, outcome and minted IDs.")

	graphqlschema.Mutation[CreateTransactionArgs, dto.Transaction](s, "createTransaction",
		"Applies the postings as one linked chain of TigerBeetle transfers: all apply or none.")
	graphqlschema.Mutation[RevertTransactionArgs, dto.Transaction](s, "revertTransaction",
		"Creates a new transaction with every posting reversed. It can be declined; a transaction is reverted at most once.")
	graphqlschema.Mutation[ReplaceTransactionMetadataArgs, dto.Transaction](s, "replaceTransactionMetadata",
		"Replaces the metadata stored with a transaction. The postings never change.")
	graphqlschema.Mutation[CreateHoldArgs, dto.Hold](s, "createHold",
		"Reserves an amount with a TigerBeetle pending transfer (ADR-0010). Nothing moves until the hold is captured.")
	graphqlschema.Mutation[CaptureHoldArgs, dto.Hold](s, "captureHold",
		"Posts the pending transfer: the full hold without an amount, or that much with the rest released.")
	graphqlschema.Mutation[VoidHoldArgs, dto.Hold](s, "voidHold", "Voids the pending transfer and releases the whole amount to the source.")
	graphqlschema.Mutation[ReplaceAccountMetadataArgs, dto.Account](s, "replaceAccountMetadata", "Replaces the metadata of an existing account.")

	graphqlschema.Field[graphqlschema.NoArgs, []dto.AssetSummary](s, "Ledger", "assets",
		"The assets used so far, with their TigerBeetle ledger numbers and the money issued.")
	graphqlschema.Field[graphqlschema.NoArgs, dto.LedgerStats](s, "Ledger", "stats",
		"Counts and amounts of the ledger overview, read from Postgres and TigerBeetle.")
	graphqlschema.Field[AccountsArgs, AccountConnection](s, "Ledger", "accounts",
		"Accounts in address order, with their balance per asset read from TigerBeetle.")
	graphqlschema.Field[AccountArgs, *dto.Account](s, "Ledger", "account", "An account by address, or null when the address has no account.")
	graphqlschema.Field[AccountHistoryArgs, dto.BalanceHistory](s, "Ledger", "accountHistory",
		"Point-in-time balances of one account, newest first. Only accounts created with the history flag keep them.")
	graphqlschema.Field[SegmentsArgs, []dto.Segment](s, "Ledger", "segments",
		"The next address segment under a prefix, with the accounts below each one; the top level without a prefix.")
	graphqlschema.Field[AggregateBalancesArgs, dto.AggregateBalances](s, "Ledger", "aggregateBalances",
		"The balances of every account under an address prefix, summed per asset.")
	graphqlschema.Field[TransactionsArgs, TransactionConnection](s, "Ledger", "transactions", "Committed transactions, newest first.")
	graphqlschema.Field[IDArgs, *dto.Transaction](s, "Ledger", "transaction", "A transaction by ID, or null.")
	graphqlschema.Field[HoldsArgs, HoldConnection](s, "Ledger", "holds", "Holds, newest first.")
	graphqlschema.Field[graphqlschema.NoArgs, dto.HoldStats](s, "Ledger", "holdCounts", "The holds of the ledger counted by status.")
	graphqlschema.Field[IDArgs, *dto.Hold](s, "Ledger", "hold", "A hold by ID, or null.")
	graphqlschema.Field[RequestsArgs, WriteRequestConnection](s, "Ledger", "requests",
		"The idempotency records of the ledger, newest first: the only place declined and in-flight writes are visible.")
	graphqlschema.Field[RequestArgs, *dto.WriteRequestDetail](s, "Ledger", "request", "A write request by Idempotency-Key, or null.")
	graphqlschema.Field[SearchArgs, dto.SearchResults](s, "Ledger", "search",
		"What a query names: an ID, an address or prefix, a reference or an Idempotency-Key.")
}

// AccountConnection is a page of accounts.
type AccountConnection struct {
	Nodes    []dto.AccountSummary   `json:"nodes"`
	PageInfo graphqlschema.PageInfo `json:"pageInfo"`
}

// TransactionConnection is a page of transactions.
type TransactionConnection struct {
	Nodes    []dto.Transaction      `json:"nodes"`
	PageInfo graphqlschema.PageInfo `json:"pageInfo"`
}

// HoldConnection is a page of holds.
type HoldConnection struct {
	Nodes    []dto.Hold             `json:"nodes"`
	PageInfo graphqlschema.PageInfo `json:"pageInfo"`
}

// WriteRequestConnection is a page of write requests.
type WriteRequestConnection struct {
	Nodes    []dto.WriteRequest     `json:"nodes"`
	PageInfo graphqlschema.PageInfo `json:"pageInfo"`
}

// AccountsArgs are the arguments of Ledger.accounts.
type AccountsArgs struct {
	First  *int                  `json:"first" default:"25" doc:"Page size, 1 to 100."`
	After  *string               `json:"after" doc:"The endCursor of the previous page."`
	Prefix *scalar.AddressPrefix `json:"prefix" doc:"Only addresses equal to or below this prefix, on whole segments."`
	Search *string               `json:"search" doc:"Only addresses starting with this text."`
	Asset  *scalar.Asset         `json:"asset" doc:"Only accounts holding this asset."`
}

// AccountArgs names an account.
type AccountArgs struct {
	Address scalar.Address `json:"address"`
}

// AccountHistoryArgs are the arguments of Ledger.accountHistory.
type AccountHistoryArgs struct {
	Address scalar.Address `json:"address"`
	Asset   scalar.Asset   `json:"asset"`
	Limit   *int           `json:"limit" default:"50" doc:"1 to 200."`
}

// SegmentsArgs are the arguments of Ledger.segments.
type SegmentsArgs struct {
	Prefix *scalar.AddressPrefix `json:"prefix"`
}

// AggregateBalancesArgs are the arguments of Ledger.aggregateBalances.
type AggregateBalancesArgs struct {
	Address scalar.AddressPrefix `json:"address" doc:"The prefix, on whole segments."`
}

// TransactionsArgs are the arguments of Ledger.transactions.
type TransactionsArgs struct {
	First           *int                  `json:"first" default:"25" doc:"Page size, 1 to 100."`
	After           *string               `json:"after" doc:"The endCursor of the previous page."`
	Address         *scalar.AddressPrefix `json:"address" doc:"Only transactions with a posting from or to an account at or below this prefix."`
	Asset           *scalar.Asset         `json:"asset"`
	Reference       *string               `json:"reference"`
	EventType       *string               `json:"eventType"`
	CommittedAfter  *time.Time            `json:"committedAfter" doc:"Committed at or after this time."`
	CommittedBefore *time.Time            `json:"committedBefore" doc:"Committed before this time."`
}

// IDArgs names a transaction or hold.
type IDArgs struct {
	ID scalar.ID `json:"id"`
}

// HoldsArgs are the arguments of Ledger.holds.
type HoldsArgs struct {
	First   *int                  `json:"first" default:"25" doc:"Page size, 1 to 100."`
	After   *string               `json:"after" doc:"The endCursor of the previous page."`
	Status  *scalar.HoldStatus    `json:"status"`
	Address *scalar.AddressPrefix `json:"address" doc:"Only holds from or to an account at or below this prefix."`
}

// RequestsArgs are the arguments of Ledger.requests.
type RequestsArgs struct {
	First     *int                  `json:"first" default:"25" doc:"Page size, 1 to 100."`
	After     *string               `json:"after" doc:"The endCursor of the previous page."`
	Status    *scalar.RequestStatus `json:"status"`
	Operation *scalar.Operation     `json:"operation"`
	Search    *string               `json:"search" doc:"An Idempotency-Key, or the start of one, or a request ID."`
}

// RequestArgs names a write request.
type RequestArgs struct {
	Key string `json:"key" doc:"The Idempotency-Key."`
}

// SearchArgs are the arguments of Ledger.search.
type SearchArgs struct {
	Q string `json:"q" doc:"1 to 255 characters."`
}

// PostingInput is the input form of dto.Posting. GraphQL forbids one type name for an input and
// an output, so the input side gets its own name over the same fields.
type PostingInput dto.Posting

// CreateTransactionInput is the REST body plus the ledger and the Idempotency-Key.
type CreateTransactionInput struct {
	Ledger         scalar.LedgerName `json:"ledger"`
	IdempotencyKey string            `json:"idempotencyKey" doc:"Makes the write safe to retry: the same key and input replay the first answer (ADR-0007)."`
	Postings       []PostingInput    `json:"postings" doc:"1 to 64 postings, applied atomically."`
	Reference      *string           `json:"reference"`
	EventType      *string           `json:"eventType" doc:"An event type name of the ledger; the default event type when absent."`
	BusinessTime   *time.Time        `json:"businessTime" doc:"The client's business time, stored in user_data_64. Not the committed time."`
	Metadata       *scalar.Metadata  `json:"metadata"`
}

// CreateTransactionArgs follows the GraphQL convention of one input argument per mutation.
type CreateTransactionArgs struct {
	Input CreateTransactionInput `json:"input"`
}

// RevertTransactionInput names the transaction to revert.
type RevertTransactionInput struct {
	Ledger         scalar.LedgerName `json:"ledger"`
	ID             scalar.ID         `json:"id"`
	IdempotencyKey string            `json:"idempotencyKey"`
	Reference      *string           `json:"reference"`
	Metadata       *scalar.Metadata  `json:"metadata"`
}

// RevertTransactionArgs wraps the input.
type RevertTransactionArgs struct {
	Input RevertTransactionInput `json:"input"`
}

// ReplaceTransactionMetadataInput is the input of replaceTransactionMetadata.
type ReplaceTransactionMetadataInput struct {
	Ledger   scalar.LedgerName `json:"ledger"`
	ID       scalar.ID         `json:"id"`
	Metadata scalar.Metadata   `json:"metadata"`
}

// ReplaceTransactionMetadataArgs wraps the input.
type ReplaceTransactionMetadataArgs struct {
	Input ReplaceTransactionMetadataInput `json:"input"`
}

// CreateHoldInput is the REST body of a hold plus the ledger and the Idempotency-Key.
type CreateHoldInput struct {
	Ledger         scalar.LedgerName `json:"ledger"`
	IdempotencyKey string            `json:"idempotencyKey"`
	Source         scalar.Address    `json:"source"`
	Destination    scalar.Address    `json:"destination"`
	Asset          scalar.Asset      `json:"asset"`
	Amount         scalar.Amount     `json:"amount"`
	TimeoutSeconds *int64            `json:"timeoutSeconds" doc:"Releases the hold after this many seconds on the cluster clock; 0 or absent means never."`
	Reference      *string           `json:"reference"`
	EventType      *string           `json:"eventType"`
	Metadata       *scalar.Metadata  `json:"metadata"`
}

// CreateHoldArgs wraps the input.
type CreateHoldArgs struct {
	Input CreateHoldInput `json:"input"`
}

// CaptureHoldInput names the hold to capture.
type CaptureHoldInput struct {
	Ledger         scalar.LedgerName `json:"ledger"`
	ID             scalar.ID         `json:"id"`
	IdempotencyKey string            `json:"idempotencyKey"`
	Amount         *scalar.Amount    `json:"amount" doc:"The amount to capture; the full hold when absent. Must not be zero."`
}

// CaptureHoldArgs wraps the input.
type CaptureHoldArgs struct {
	Input CaptureHoldInput `json:"input"`
}

// VoidHoldInput names the hold to void.
type VoidHoldInput struct {
	Ledger         scalar.LedgerName `json:"ledger"`
	ID             scalar.ID         `json:"id"`
	IdempotencyKey string            `json:"idempotencyKey"`
}

// VoidHoldArgs wraps the input.
type VoidHoldArgs struct {
	Input VoidHoldInput `json:"input"`
}

// ReplaceAccountMetadataInput is the input of replaceAccountMetadata.
type ReplaceAccountMetadataInput struct {
	Ledger   scalar.LedgerName `json:"ledger"`
	Address  scalar.Address    `json:"address"`
	Metadata scalar.Metadata   `json:"metadata"`
}

// ReplaceAccountMetadataArgs wraps the input.
type ReplaceAccountMetadataArgs struct {
	Input ReplaceAccountMetadataInput `json:"input"`
}
