// Package domain is the domain layer of the bookkeeping module (plan section 4.6): postings, write
// operations and holds, the classification of a TigerBeetle chain, the stored outcome of a write,
// and the ports its infrastructure implements, with the records they exchange. It imports only the
// shared kernel.
package domain

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// LedgerStore is the TigerBeetle port: the only store of balances and transfers (ADR-0002). The
// adapter maps TigerBeetle results to the Result* strings below and client failures to
// ledger.ErrUnavailable.
type LedgerStore interface {
	// Ready performs a round trip to the cluster and fails when it does not answer before the
	// context deadline.
	Ready(ctx context.Context) error
	// CreateAccounts creates accounts, treating "exists" as created. Any other result is an
	// invariant violation (ADR-0006) and returns an error that is not a client error.
	CreateAccounts(ctx context.Context, accounts []AccountSpec) error
	// CreateTransfers submits transfers in one request and returns one outcome per transfer, in
	// order. A context deadline returns ledger.ErrUnavailable; the request may still land, which is
	// what the reconciler is for.
	CreateTransfers(ctx context.Context, transfers []TransferSpec) ([]TransferOutcome, error)
	// LookupAccounts returns the accounts that exist, in any order.
	LookupAccounts(ctx context.Context, ids []ledger.ID) ([]AccountState, error)
	// LookupTransfers returns the transfers that exist, in any order.
	LookupTransfers(ctx context.Context, ids []ledger.ID) ([]TransferRecord, error)
	// AccountBalances returns point-in-time balances, newest first, for accounts with the history
	// flag.
	AccountBalances(ctx context.Context, id ledger.ID, limit int) ([]BalancePoint, error)
}

// AccountRepository stores addresses, the TigerBeetle accounts behind them and the ledger number of
// every (ledger, asset) pair.
type AccountRepository interface {
	// EnsureAssets allocates the TigerBeetle ledger number of each asset on first use (ADR-0005)
	// and returns the rows, keyed by canonical asset.
	EnsureAssets(ctx context.Context, ledgerName string, assets []ledger.Asset) (map[string]AssetRow, error)
	// FindAccounts returns the existing accounts among the given (address, asset) pairs.
	FindAccounts(ctx context.Context, ledgerName string, keys []AccountKey) ([]AccountRow, error)
	// EnsureAccounts inserts the candidate rows with ON CONFLICT DO NOTHING and returns the rows
	// that won, so concurrent first uses converge on one account (ADR-0006).
	EnsureAccounts(ctx context.Context, ledgerName string, candidates []AccountRow) ([]AccountRow, error)
	// AccountsByID returns the accounts with the given IDs.
	AccountsByID(ctx context.Context, ids []ledger.ID) ([]AccountRow, error)
	// ListAddresses returns a page of addresses with their accounts, in address order.
	ListAddresses(ctx context.Context, ledgerName string, filter AddressFilter, after string, limit int) ([]AddressRow, error)
	// GetAddress returns ledger.ErrNotFound when the address has no account.
	GetAddress(ctx context.Context, ledgerName string, address ledger.Address) (AddressRow, error)
	ReplaceAddressMetadata(ctx context.Context, ledgerName string, address ledger.Address, metadata map[string]string) error
	// AccountsUnder returns the accounts at or below a prefix, at most limit of them.
	AccountsUnder(ctx context.Context, ledgerName string, prefix ledger.Address, limit int) ([]AccountRow, error)
	// Segments returns the next segment below a prefix (or the top level for an empty prefix).
	Segments(ctx context.Context, ledgerName string, prefix ledger.Address) ([]SegmentRow, error)
	ListAssets(ctx context.Context, ledgerName string) ([]AssetRow, error)
	// LedgerFootprints returns the assets and address count of each named ledger, one entry per
	// name, for the ledgers module.
	LedgerFootprints(ctx context.Context, ledgerNames []string) (map[string]ledger.Footprint, error)
	// CountAddresses returns the number of addresses and those created since the given time.
	CountAddresses(ctx context.Context, ledgerName string, since time.Time) (total, recent int64, err error)
	// SearchAddresses returns addresses starting with the text, for search.
	SearchAddresses(ctx context.Context, ledgerName, text string, limit int) ([]AddressRow, error)
}

// WriteRepository stores idempotency records (ADR-0007).
type WriteRepository interface {
	// BeginWrite inserts a pending record. When a record with the same key exists it returns that
	// record and created false. A second revert of one transaction, or a second resolution of one
	// hold, returns ledger.ErrConflict.
	BeginWrite(ctx context.Context, record WriteRecord) (WriteRecord, bool, error)
	GetWrite(ctx context.Context, ledgerName, key string) (WriteRecord, error)
	ListWrites(ctx context.Context, ledgerName string, filter WriteFilter, cursor WriteCursor, limit int) ([]WriteRecord, error)
	// NoteAttempt increments the submission counter before a TigerBeetle call.
	NoteAttempt(ctx context.Context, ledgerName, key string) error
	// ResolveWrite records a declined or failed outcome.
	ResolveWrite(ctx context.Context, ledgerName, key string, outcome WriteOutcome) error
	// CommitTransaction marks the record committed and inserts the transaction in one Postgres
	// transaction.
	CommitTransaction(ctx context.Context, outcome WriteOutcome, tx TransactionRow) error
	// CommitHold marks the record committed and inserts the hold.
	CommitHold(ctx context.Context, outcome WriteOutcome, hold HoldRow) error
	// CommitResolution marks the record committed and resolves the hold.
	CommitResolution(ctx context.Context, outcome WriteOutcome, resolution HoldResolution) error
	// PendingWrites returns pending records created before the given time, oldest first.
	PendingWrites(ctx context.Context, before time.Time, limit int) ([]WriteRecord, error)
	// CountPending returns the pending records of the organization of the call and those created
	// before the given time.
	CountPending(ctx context.Context, before time.Time) (pending, stuck int64, err error)
	// DeclinedSince groups declined and failed records of a ledger by problem type.
	DeclinedSince(ctx context.Context, ledgerName string, since time.Time) ([]ReasonCount, error)
}

// TransactionRepository reads committed transactions.
type TransactionRepository interface {
	GetTransaction(ctx context.Context, ledgerName string, id ledger.ID) (TransactionRow, error)
	ListTransactions(ctx context.Context, ledgerName string, filter TransactionFilter, before ledger.ID, limit int) ([]TransactionRow, error)
	ReplaceTransactionMetadata(ctx context.Context, ledgerName string, id ledger.ID, metadata map[string]string) error
	// SearchTransactions finds transactions by exact ID or by reference prefix.
	SearchTransactions(ctx context.Context, ledgerName, text string, limit int) ([]TransactionRow, error)
	// EventTypeUsage counts transactions and holds per event type name.
	EventTypeUsage(ctx context.Context, ledgerName string) (map[string]int64, error)
}

// HoldRepository reads holds and marks expired ones.
type HoldRepository interface {
	GetHold(ctx context.Context, ledgerName string, id ledger.ID) (HoldRow, error)
	// ListHolds filters by visible status; open and expired are computed against now.
	ListHolds(ctx context.Context, ledgerName string, filter HoldFilter, before ledger.ID, limit int, now time.Time) ([]HoldRow, error)
	CountHolds(ctx context.Context, ledgerName string, now time.Time) (HoldCounts, error)
	// OpenHolds returns open, unexpired holds, at most limit.
	OpenHolds(ctx context.Context, ledgerName string, now time.Time, limit int) ([]HoldRow, error)
	// MarkHoldExpired records that TigerBeetle answered pending_transfer_expired.
	MarkHoldExpired(ctx context.Context, ledgerName string, id ledger.ID) error
	CountOpenHoldsFrom(ctx context.Context, ledgerName string, address ledger.Address, now time.Time) (int64, error)
}

// IDGenerator mints time-ordered 128-bit identifiers. The TigerBeetle adapter implements it with
// the client's ID() helper (ADR-0006).
type IDGenerator interface {
	NewID() ledger.ID
}
