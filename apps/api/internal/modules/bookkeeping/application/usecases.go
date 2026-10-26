package application

import (
	"context"
	"math/big"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// TransactionCommands runs the transaction write path of ADR-0007.
type TransactionCommands interface {
	// Create applies the postings as one linked chain: all or nothing.
	Create(ctx context.Context, in CreateTransactionInput) (TransactionView, error)
	// Revert creates a new transaction with every posting reversed. It is a normal chain and can be
	// declined; there is no force option (ADR-0004).
	Revert(ctx context.Context, in RevertTransactionInput) (TransactionView, error)
	// ReplaceMetadata replaces the metadata stored with a transaction; the postings never change.
	ReplaceMetadata(ctx context.Context, ledgerName, id string, metadata map[string]string) (TransactionView, error)
}

// TransactionQueries reads committed transactions, with their amounts read from TigerBeetle.
type TransactionQueries interface {
	Get(ctx context.Context, ledgerName, id string) (TransactionView, error)
	// List returns committed transactions newest first and the cursor of the next page.
	List(ctx context.Context, in ListTransactionsInput) ([]TransactionView, string, error)
}

// HoldCommands runs the two-phase transfer flows of ADR-0010 on the write path of ADR-0007.
type HoldCommands interface {
	// Create reserves the amount with a pending transfer.
	Create(ctx context.Context, in CreateHoldInput) (HoldView, error)
	// Capture posts the pending transfer: the full amount, or a smaller amount with the rest
	// released. A hold resolves exactly once.
	Capture(ctx context.Context, in CaptureHoldInput) (HoldView, error)
	// Void releases the whole hold back to the source.
	Void(ctx context.Context, in VoidHoldInput) (HoldView, error)
}

// HoldQueries reads holds, with their amounts read from TigerBeetle.
type HoldQueries interface {
	Get(ctx context.Context, ledgerName, id string) (HoldView, error)
	// List returns holds newest first, the cursor of the next page and the count per status.
	List(ctx context.Context, in ListHoldsInput) ([]HoldView, string, domain.HoldCounts, error)
	// Counts returns the count of holds per visible status.
	Counts(ctx context.Context, ledgerName string) (domain.HoldCounts, error)
}

// AccountCommands edits accounts. Accounts are created by the writes that first name them.
type AccountCommands interface {
	// ReplaceMetadata replaces the metadata of an existing address.
	ReplaceMetadata(ctx context.Context, ledgerName, address string, metadata map[string]string) (AccountView, error)
}

// AccountQueries reads accounts and balances. Postgres resolves names to IDs; TigerBeetle answers
// every amount (ADR-0002).
type AccountQueries interface {
	// List returns a page of accounts with their balances, in address order.
	List(ctx context.Context, in ListAccountsInput) ([]AccountView, string, error)
	// Get returns one account with its balances and open hold count.
	Get(ctx context.Context, ledgerName, address string) (AccountView, error)
	// History returns point-in-time balances of the account for one asset (ADR-0006).
	History(ctx context.Context, ledgerName, address, asset string, limit int) ([]HistoryPoint, error)
	// AggregateBalances sums the balances of every account under a prefix, per asset (ADR-0004).
	AggregateBalances(ctx context.Context, ledgerName, prefix string) (Aggregate, error)
	// Segments returns the prefix it read and the chart of accounts one level below it.
	Segments(ctx context.Context, ledgerName, prefix string) (string, []domain.SegmentRow, error)
	// Assets lists the assets of a ledger with the money issued.
	Assets(ctx context.Context, ledgerName string) ([]AssetView, error)
}

// RequestQueries reads the idempotency records of a ledger.
type RequestQueries interface {
	// List returns records newest first and the cursor of the next page.
	List(ctx context.Context, in ListRequestsInput) ([]domain.WriteRecord, string, error)
	Get(ctx context.Context, ledgerName, key string) (domain.WriteRecord, error)
}

// OverviewQueries answers the ledger overview and search.
type OverviewQueries interface {
	Stats(ctx context.Context, ledgerName string) (Stats, error)
	// Search finds accounts by address prefix, transactions by ID or reference, holds by ID, and
	// write requests by Idempotency-Key.
	Search(ctx context.Context, ledgerName, query string) (SearchResult, error)
}

// ReconcilerQueries reports the reconciler.
type ReconcilerQueries interface {
	Status(ctx context.Context) (ReconcilerStatus, error)
}

// CreateTransactionInput is a transaction request, whatever transport it came through.
type CreateTransactionInput struct {
	Ledger       string
	Key          string
	RequestID    string
	Postings     []domain.Posting
	Reference    string
	EventType    string
	BusinessTime *time.Time
	Metadata     map[string]string
}

// RevertTransactionInput names the transaction to revert.
type RevertTransactionInput struct {
	Ledger    string
	Key       string
	RequestID string
	ID        string
	Reference string
	Metadata  map[string]string
}

// ListTransactionsInput is the transaction list query.
type ListTransactionsInput struct {
	Ledger    string
	Address   string
	Asset     string
	Reference string
	EventType string
	After     *time.Time
	Before    *time.Time
	Cursor    string
	Limit     int
}

// CreateHoldInput is a hold request.
type CreateHoldInput struct {
	Ledger         string
	Key            string
	RequestID      string
	Posting        domain.Posting
	TimeoutSeconds uint32
	Reference      string
	EventType      string
	Metadata       map[string]string
}

// CaptureHoldInput names the hold to capture; a nil amount captures all of it.
type CaptureHoldInput struct {
	Ledger    string
	Key       string
	RequestID string
	ID        string
	Amount    *ledger.Amount
}

// VoidHoldInput names the hold to void.
type VoidHoldInput struct {
	Ledger    string
	Key       string
	RequestID string
	ID        string
}

// ListHoldsInput is the hold list query.
type ListHoldsInput struct {
	Ledger  string
	Status  string
	Address string
	Cursor  string
	Limit   int
}

// ListAccountsInput is the account list query.
type ListAccountsInput struct {
	Ledger string
	Prefix string
	Search string
	Asset  string
	Cursor string
	Limit  int
}

// ListRequestsInput is the write request list query.
type ListRequestsInput struct {
	Ledger    string
	Status    string
	Operation string
	Search    string
	Cursor    string
	Limit     int
}

// PostingView is a posting of a committed transaction with the amount read from TigerBeetle.
type PostingView struct {
	Source      ledger.Address
	Destination ledger.Address
	Asset       string
	Amount      ledger.Amount
	TransferID  ledger.ID
	TBLedger    uint32
}

// TransactionView is a committed transaction.
type TransactionView struct {
	ID             ledger.ID
	Ledger         string
	Postings       []PostingView
	Reference      string
	EventType      ledger.EventType
	BusinessTime   *time.Time
	Metadata       map[string]string
	CommittedAt    time.Time
	Timestamp      uint64
	Reverts        *ledger.ID
	RevertedBy     *ledger.ID
	IdempotencyKey string
	RequestID      string
}

// HoldView is a hold with its amounts read from TigerBeetle (ADR-0010).
type HoldView struct {
	ID                   ledger.ID
	Ledger               string
	Source               ledger.Address
	Destination          ledger.Address
	Asset                string
	Amount               ledger.Amount
	CapturedAmount       *ledger.Amount
	Status               domain.HoldStatus
	TimeoutSeconds       uint32
	CreatedAt            time.Time
	Timestamp            uint64
	ExpiresAt            *time.Time
	ResolvedAt           *time.Time
	Reference            string
	EventType            ledger.EventType
	Metadata             map[string]string
	IdempotencyKey       string
	PendingTransferID    ledger.ID
	ResolutionTransferID *ledger.ID
	TBLedger             uint32
}

// BalanceView is one TigerBeetle account of an address with its balances read from the cluster.
type BalanceView struct {
	domain.AccountRow
	// Balance is credits posted minus debits posted; it may be negative (world).
	Balance *big.Int
	// Available is the balance minus pending debits, the amount TigerBeetle lets the account spend
	// under debits_must_not_exceed_credits (ADR-0010).
	Available *big.Int
	Reserved  ledger.Amount
	Incoming  ledger.Amount
	Credits   ledger.Amount
	Debits    ledger.Amount
}

// AccountView is an address with one balance per asset.
type AccountView struct {
	Address   ledger.Address
	Metadata  map[string]string
	CreatedAt time.Time
	Balances  []BalanceView
	OpenHolds int64
}

// HistoryPoint is one historical balance.
type HistoryPoint struct {
	Timestamp uint64
	Balance   *big.Int
	Reserved  ledger.Amount
	Credits   ledger.Amount
	Debits    ledger.Amount
}

// AggregateAsset is the sum of every account of one asset under a prefix.
type AggregateAsset struct {
	Asset     string
	Accounts  int64
	Balance   *big.Int
	Available *big.Int
	Reserved  *big.Int
	Incoming  *big.Int
}

// Aggregate is a prefix sum.
type Aggregate struct {
	Prefix    string
	AsOf      time.Time
	Addresses int64
	Chunks    int
	Assets    []AggregateAsset
}

// AssetView is an asset of a ledger with the money issued, minus the balance of world.
type AssetView struct {
	domain.AssetRow
	Issued *big.Int
}

// Stats are the overview figures of a ledger.
type Stats struct {
	Accounts      int64
	AccountsToday int64
	Assets        []AssetView
	Holds         domain.HoldCounts
	// Reserved is the amount held by open holds, per asset, read from TigerBeetle.
	Reserved      map[string]*big.Int
	DeclinedTotal int64
	Declined      []domain.ReasonCount
}

// SearchHold is a hold a search found, with its status now.
type SearchHold struct {
	ID     ledger.ID
	Status domain.HoldStatus
}

// SearchResult groups what a query names.
type SearchResult struct {
	Accounts     []domain.AddressRow
	Transactions []domain.TransactionRow
	Holds        []SearchHold
	Requests     []domain.WriteRecord
}

// StuckRecord is a pending record the reconciler could not settle.
type StuckRecord struct {
	// Organization is the record's tenant; Status shows each organization its own records.
	Organization ledger.ID
	Ledger       string
	Key          string
	Operation    domain.Operation
	CreatedAt    time.Time
	Attempts     int
	Note         string
}

// ReconcilerStatus is what the System page shows.
type ReconcilerStatus struct {
	Running     bool
	Grace       time.Duration
	Interval    time.Duration
	LastRunAt   time.Time
	LastSettled int
	Pending     int64
	Stuck       int64
	Records     []StuckRecord
}
