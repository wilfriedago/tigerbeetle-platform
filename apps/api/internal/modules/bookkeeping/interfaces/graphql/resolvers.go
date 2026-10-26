package graphql

import (
	"context"
	"errors"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/graphqlschema"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/requestid"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/scalar"
)

// Mutations holds the bookkeeping mutation resolvers.
type Mutations struct {
	app application.Application
}

// NewMutations builds the mutation resolvers.
func NewMutations(app application.Application) *Mutations {
	return &Mutations{app: app}
}

// CreateTransaction resolves Mutation.createTransaction. The use case is the one REST calls, so a
// write retried over the other transport with the same key replays (plan section 10).
func (m *Mutations) CreateTransaction(ctx context.Context, input CreateTransactionInput) (*dto.Transaction, error) {
	postings, err := postingsIn(input.Postings)
	if err != nil {
		return nil, err
	}
	v, err := m.app.TransactionCommands.Create(ctx, application.CreateTransactionInput{
		Ledger:       string(input.Ledger),
		Key:          input.IdempotencyKey,
		RequestID:    requestid.From(ctx),
		Postings:     postings,
		Reference:    deref(input.Reference),
		EventType:    deref(input.EventType),
		BusinessTime: input.BusinessTime,
		Metadata:     metadataIn(input.Metadata),
	})
	if err != nil {
		return nil, err
	}
	out := dto.TransactionFrom(&v)
	return &out, nil
}

// RevertTransaction resolves Mutation.revertTransaction.
func (m *Mutations) RevertTransaction(ctx context.Context, input RevertTransactionInput) (*dto.Transaction, error) {
	v, err := m.app.TransactionCommands.Revert(ctx, application.RevertTransactionInput{
		Ledger:    string(input.Ledger),
		Key:       input.IdempotencyKey,
		RequestID: requestid.From(ctx),
		ID:        string(input.ID),
		Reference: deref(input.Reference),
		Metadata:  metadataIn(input.Metadata),
	})
	if err != nil {
		return nil, err
	}
	out := dto.TransactionFrom(&v)
	return &out, nil
}

// ReplaceTransactionMetadata resolves Mutation.replaceTransactionMetadata.
func (m *Mutations) ReplaceTransactionMetadata(ctx context.Context, input ReplaceTransactionMetadataInput) (*dto.Transaction, error) {
	v, err := m.app.TransactionCommands.ReplaceMetadata(ctx, string(input.Ledger), string(input.ID), input.Metadata.Map())
	if err != nil {
		return nil, err
	}
	out := dto.TransactionFrom(&v)
	return &out, nil
}

// CreateHold resolves Mutation.createHold.
func (m *Mutations) CreateHold(ctx context.Context, input CreateHoldInput) (*dto.Hold, error) {
	posting, err := dto.Posting{Source: input.Source, Destination: input.Destination, Asset: input.Asset, Amount: input.Amount}.Parse()
	if err != nil {
		return nil, err
	}
	timeout := deref(input.TimeoutSeconds)
	if timeout < 0 || timeout > domain.MaxHoldTimeout {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "timeoutSeconds must be between 0 and 4294967295")
	}
	v, err := m.app.HoldCommands.Create(ctx, application.CreateHoldInput{
		Ledger:         string(input.Ledger),
		Key:            input.IdempotencyKey,
		RequestID:      requestid.From(ctx),
		Posting:        posting,
		TimeoutSeconds: uint32(timeout),
		Reference:      deref(input.Reference),
		EventType:      deref(input.EventType),
		Metadata:       metadataIn(input.Metadata),
	})
	if err != nil {
		return nil, err
	}
	out := dto.HoldFrom(&v)
	return &out, nil
}

// CaptureHold resolves Mutation.captureHold.
func (m *Mutations) CaptureHold(ctx context.Context, input CaptureHoldInput) (*dto.Hold, error) {
	capture := application.CaptureHoldInput{Ledger: string(input.Ledger), Key: input.IdempotencyKey, RequestID: requestid.From(ctx), ID: string(input.ID)}
	if input.Amount != nil {
		a, err := ledger.ParseAmount(string(*input.Amount))
		if err != nil {
			return nil, err
		}
		capture.Amount = &a
	}
	v, err := m.app.HoldCommands.Capture(ctx, capture)
	if err != nil {
		return nil, err
	}
	out := dto.HoldFrom(&v)
	return &out, nil
}

// VoidHold resolves Mutation.voidHold.
func (m *Mutations) VoidHold(ctx context.Context, input VoidHoldInput) (*dto.Hold, error) {
	v, err := m.app.HoldCommands.Void(ctx, application.VoidHoldInput{
		Ledger: string(input.Ledger), Key: input.IdempotencyKey, RequestID: requestid.From(ctx), ID: string(input.ID),
	})
	if err != nil {
		return nil, err
	}
	out := dto.HoldFrom(&v)
	return &out, nil
}

// ReplaceAccountMetadata resolves Mutation.replaceAccountMetadata.
func (m *Mutations) ReplaceAccountMetadata(ctx context.Context, input ReplaceAccountMetadataInput) (*dto.Account, error) {
	v, err := m.app.AccountCommands.ReplaceMetadata(ctx, string(input.Ledger), string(input.Address), input.Metadata.Map())
	if err != nil {
		return nil, err
	}
	out := dto.AccountFrom(&v)
	return &out, nil
}

// LedgerFields resolves the fields bookkeeping adds to Ledger. Ledger belongs to the ledgers
// module, so these methods take the ledger name; internal/transport/graphql passes it from the
// parent object (plan section 8.9).
type LedgerFields struct {
	app application.Application
}

// NewLedgerFields builds the field resolvers.
func NewLedgerFields(app application.Application) *LedgerFields {
	return &LedgerFields{app: app}
}

// Assets resolves Ledger.assets.
func (f *LedgerFields) Assets(ctx context.Context, ledgerName string) ([]dto.AssetSummary, error) {
	assets, err := f.app.AccountQueries.Assets(ctx, ledgerName)
	if err != nil {
		return nil, err
	}
	return dto.AssetSummariesFrom(assets), nil
}

// Stats resolves Ledger.stats.
func (f *LedgerFields) Stats(ctx context.Context, ledgerName string) (*dto.LedgerStats, error) {
	st, err := f.app.OverviewQueries.Stats(ctx, ledgerName)
	if err != nil {
		return nil, err
	}
	out := dto.LedgerStatsFrom(&st)
	return &out, nil
}

// Accounts resolves Ledger.accounts.
func (f *LedgerFields) Accounts(ctx context.Context, ledgerName string, first *int, after, prefix, search, asset *string) (*AccountConnection, error) {
	views, next, err := f.app.AccountQueries.List(ctx, application.ListAccountsInput{
		Ledger: ledgerName, Prefix: deref(prefix), Search: deref(search), Asset: deref(asset), Cursor: deref(after), Limit: deref(first),
	})
	if err != nil {
		return nil, err
	}
	out := &AccountConnection{Nodes: make([]dto.AccountSummary, len(views)), PageInfo: graphqlschema.PageInfoOf(next)}
	for i := range views {
		out.Nodes[i] = dto.AccountSummaryFrom(&views[i])
	}
	return out, nil
}

// Account resolves Ledger.account: null when the address has no account.
func (f *LedgerFields) Account(ctx context.Context, ledgerName, address string) (*dto.Account, error) {
	v, err := f.app.AccountQueries.Get(ctx, ledgerName, address)
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := dto.AccountFrom(&v)
	return &out, nil
}

// AccountHistory resolves Ledger.accountHistory.
func (f *LedgerFields) AccountHistory(ctx context.Context, ledgerName, address, asset string, limit *int) (*dto.BalanceHistory, error) {
	points, err := f.app.AccountQueries.History(ctx, ledgerName, address, asset, deref(limit))
	if err != nil {
		return nil, err
	}
	out := dto.BalanceHistoryFrom(scalar.Address(address), scalar.Asset(asset), points)
	return &out, nil
}

// Segments resolves Ledger.segments.
func (f *LedgerFields) Segments(ctx context.Context, ledgerName string, prefix *string) ([]dto.Segment, error) {
	_, rows, err := f.app.AccountQueries.Segments(ctx, ledgerName, deref(prefix))
	if err != nil {
		return nil, err
	}
	out := make([]dto.Segment, len(rows))
	for i := range rows {
		out[i] = dto.SegmentFrom(&rows[i])
	}
	return out, nil
}

// AggregateBalances resolves Ledger.aggregateBalances.
func (f *LedgerFields) AggregateBalances(ctx context.Context, ledgerName, address string) (*dto.AggregateBalances, error) {
	agg, err := f.app.AccountQueries.AggregateBalances(ctx, ledgerName, address)
	if err != nil {
		return nil, err
	}
	out := dto.AggregateBalancesFrom(&agg)
	return &out, nil
}

// TransactionFilters are the filters of Ledger.transactions.
type TransactionFilters struct {
	Address, Asset, Reference, EventType *string
	CommittedAfter, CommittedBefore      *time.Time
}

// Transactions resolves Ledger.transactions.
func (f *LedgerFields) Transactions(ctx context.Context, ledgerName string, first *int, after *string, filters TransactionFilters) (*TransactionConnection, error) {
	views, next, err := f.app.TransactionQueries.List(ctx, application.ListTransactionsInput{
		Ledger:    ledgerName,
		Address:   deref(filters.Address),
		Asset:     deref(filters.Asset),
		Reference: deref(filters.Reference),
		EventType: deref(filters.EventType),
		After:     filters.CommittedAfter,
		Before:    filters.CommittedBefore,
		Cursor:    deref(after),
		Limit:     deref(first),
	})
	if err != nil {
		return nil, err
	}
	out := &TransactionConnection{Nodes: make([]dto.Transaction, len(views)), PageInfo: graphqlschema.PageInfoOf(next)}
	for i := range views {
		out.Nodes[i] = dto.TransactionFrom(&views[i])
	}
	return out, nil
}

// Transaction resolves Ledger.transaction: null when no transaction has that ID.
func (f *LedgerFields) Transaction(ctx context.Context, ledgerName string, id scalar.ID) (*dto.Transaction, error) {
	v, err := f.app.TransactionQueries.Get(ctx, ledgerName, string(id))
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := dto.TransactionFrom(&v)
	return &out, nil
}

// Holds resolves Ledger.holds.
func (f *LedgerFields) Holds(ctx context.Context, ledgerName string, first *int, after *string, status *scalar.HoldStatus, address *string) (*HoldConnection, error) {
	views, next, _, err := f.app.HoldQueries.List(ctx, application.ListHoldsInput{
		Ledger: ledgerName, Status: string(deref(status)), Address: deref(address), Cursor: deref(after), Limit: deref(first),
	})
	if err != nil {
		return nil, err
	}
	out := &HoldConnection{Nodes: make([]dto.Hold, len(views)), PageInfo: graphqlschema.PageInfoOf(next)}
	for i := range views {
		out.Nodes[i] = dto.HoldFrom(&views[i])
	}
	return out, nil
}

// HoldCounts resolves Ledger.holdCounts.
func (f *LedgerFields) HoldCounts(ctx context.Context, ledgerName string) (*dto.HoldStats, error) {
	counts, err := f.app.HoldQueries.Counts(ctx, ledgerName)
	if err != nil {
		return nil, err
	}
	out := dto.HoldStatsFrom(counts, nil)
	return &out, nil
}

// Hold resolves Ledger.hold: null when no hold has that ID.
func (f *LedgerFields) Hold(ctx context.Context, ledgerName string, id scalar.ID) (*dto.Hold, error) {
	v, err := f.app.HoldQueries.Get(ctx, ledgerName, string(id))
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := dto.HoldFrom(&v)
	return &out, nil
}

// Requests resolves Ledger.requests.
func (f *LedgerFields) Requests(ctx context.Context, ledgerName string, first *int, after *string, status *scalar.RequestStatus, operation *scalar.Operation, search, actor *string) (*WriteRequestConnection, error) {
	rows, next, err := f.app.RequestQueries.List(ctx, application.ListRequestsInput{
		Ledger: ledgerName, Status: string(deref(status)), Operation: string(deref(operation)), Search: deref(search), Actor: deref(actor),
		Cursor: deref(after), Limit: deref(first),
	})
	if err != nil {
		return nil, err
	}
	out := &WriteRequestConnection{Nodes: make([]dto.WriteRequest, len(rows)), PageInfo: graphqlschema.PageInfoOf(next)}
	for i := range rows {
		out.Nodes[i] = dto.WriteRequestFrom(&rows[i])
	}
	return out, nil
}

// Request resolves Ledger.request: null when no write used that key.
func (f *LedgerFields) Request(ctx context.Context, ledgerName, key string) (*dto.WriteRequestDetail, error) {
	rec, err := f.app.RequestQueries.Get(ctx, ledgerName, key)
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := dto.WriteRequestDetailFrom(&rec)
	return &out, nil
}

// Search resolves Ledger.search.
func (f *LedgerFields) Search(ctx context.Context, ledgerName, q string) (*dto.SearchResults, error) {
	r, err := f.app.OverviewQueries.Search(ctx, ledgerName, q)
	if err != nil {
		return nil, err
	}
	out := dto.SearchResultsFrom(q, &r)
	return &out, nil
}

// notFound reports the missing object a nullable field answers with null (plan section 8.5).
func notFound(err error) bool {
	return errors.Is(err, ledger.ErrNotFound)
}

// postingsIn parses the postings of a transaction input.
func postingsIn(in []PostingInput) ([]domain.Posting, error) {
	postings := make([]dto.Posting, len(in))
	for i, p := range in {
		postings[i] = dto.Posting(p)
	}
	return dto.ParsePostings(postings)
}

func metadataIn(m *scalar.Metadata) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m.Map()
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
