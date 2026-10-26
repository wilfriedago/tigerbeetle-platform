package domain

import (
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// AccountSpec is a TigerBeetle account to create, built only from the stored row (ADR-0006).
type AccountSpec struct {
	ID       ledger.ID
	TBLedger uint32
	Code     uint16
	Limit    ledger.Limit
	History  bool
}

// TransferKind selects the TigerBeetle transfer flags of a leg.
type TransferKind int

// Transfer kinds: a posting, a hold (pending), and the two ways to resolve a hold (ADR-0010).
const (
	TransferPosting TransferKind = iota
	TransferPending
	TransferPostPending
	TransferVoidPending
)

// TransferSpec is one TigerBeetle transfer to submit.
type TransferSpec struct {
	ID              ledger.ID
	DebitAccountID  ledger.ID
	CreditAccountID ledger.ID
	Amount          ledger.Amount
	PendingID       ledger.ID
	TBLedger        uint32
	Code            uint16
	Kind            TransferKind
	// Linked ties this transfer to the next one in the request (ADR-0007).
	Linked         bool
	TimeoutSeconds uint32
	// UserData128 carries the transaction or hold ID, so every leg points back to it.
	UserData128 ledger.ID
	// UserData64 carries the optional business time, in Unix milliseconds (ADR-0004).
	UserData64 uint64
}

// TransferOutcome is the result of one transfer of a create_transfers request.
type TransferOutcome struct {
	Result string
	// Timestamp is the TigerBeetle timestamp of a created transfer.
	Timestamp uint64
}

// AccountState is an account as TigerBeetle reports it.
type AccountState struct {
	ID             ledger.ID
	DebitsPending  ledger.Amount
	DebitsPosted   ledger.Amount
	CreditsPending ledger.Amount
	CreditsPosted  ledger.Amount
	Timestamp      uint64
}

// TransferRecord is a transfer as TigerBeetle reports it.
type TransferRecord struct {
	ID              ledger.ID
	DebitAccountID  ledger.ID
	CreditAccountID ledger.ID
	Amount          ledger.Amount
	PendingID       ledger.ID
	UserData128     ledger.ID
	UserData64      uint64
	TBLedger        uint32
	Code            uint16
	Linked          bool
	Pending         bool
	PostPending     bool
	VoidPending     bool
	TimeoutSeconds  uint32
	Timestamp       uint64
}

// BalancePoint is a historical balance of an account with the history flag.
type BalancePoint struct {
	DebitsPending  ledger.Amount
	DebitsPosted   ledger.Amount
	CreditsPending ledger.Amount
	CreditsPosted  ledger.Amount
	Timestamp      uint64
}

// AssetRow is the TigerBeetle ledger number of a (ledger, asset) pair (ADR-0005).
type AssetRow struct {
	Asset     string
	TBLedger  uint32
	CreatedAt time.Time
	Accounts  int64
}

// AccountKey identifies one TigerBeetle account by address and canonical asset.
type AccountKey struct {
	Address ledger.Address
	Asset   string
}

// AccountRow describes one TigerBeetle account, with the flags and code frozen at creation.
type AccountRow struct {
	ID          ledger.ID
	Ledger      string
	Address     ledger.Address
	Asset       string
	TBLedger    uint32
	Code        uint16
	Limit       ledger.Limit
	History     bool
	RulePattern string
	CreatedAt   time.Time
}

// Key returns the (address, asset) pair of the row.
func (r AccountRow) Key() AccountKey {
	return AccountKey{Address: r.Address, Asset: r.Asset}
}

// AddressRow is an address with its metadata and accounts.
type AddressRow struct {
	Ledger    string
	Address   ledger.Address
	Metadata  map[string]string
	CreatedAt time.Time
	Accounts  []AccountRow
}

// AddressFilter narrows an address listing.
type AddressFilter struct {
	// Prefix matches whole segments; empty means every address.
	Prefix ledger.Address
	// Search matches the start of the address as typed.
	Search string
	// Asset keeps addresses holding this canonical asset.
	Asset string
}

// SegmentRow is one child segment below a prefix.
type SegmentRow struct {
	Segment     string
	Prefix      string
	Accounts    int64
	Assets      []string
	IsAccount   bool
	HasChildren bool
}

// WriteRecord is an idempotency record (ADR-0007).
type WriteRecord struct {
	// Organization is the tenant the record belongs to (ADR-0026). The reconciler, which reads
	// records across tenants, acts for it when it settles the record.
	Organization   ledger.ID
	Ledger         string
	Key            string
	Operation      Operation
	Status         RequestStatus
	BodyHash       []byte
	Request        []byte
	TargetID       ledger.ID
	TransferIDs    []ledger.ID
	ResponseStatus int
	Response       []byte
	ProblemType    string
	RequestID      string
	Attempts       int
	CreatedAt      time.Time
	ResolvedAt     *time.Time
	// ActorIssuer and ActorSubject name the principal that made the write (ADR-0030); a key
	// replays only for it. Records from before authentication have neither.
	ActorIssuer  string
	ActorSubject string
}

// WriteOutcome is how a pending record resolves.
type WriteOutcome struct {
	Ledger         string
	Key            string
	Status         RequestStatus
	ResponseStatus int
	Response       []byte
	ProblemType    string
	ResolvedAt     time.Time
}

// WriteFilter narrows the write request listing.
type WriteFilter struct {
	Status    RequestStatus
	Operation Operation
	Search    string
}

// WriteCursor is the position after which the next page of write requests starts.
type WriteCursor struct {
	CreatedAt time.Time
	Key       string
}

// ReasonCount is a problem type and how often it occurred.
type ReasonCount struct {
	Reason string
	Count  int64
}

// LegRow is one posting of a stored transaction: its transfer and its accounts, never the amount.
type LegRow struct {
	TransferID  ledger.ID
	Source      AccountRow
	Destination AccountRow
}

// TransactionRow is a committed transaction as Postgres stores it.
type TransactionRow struct {
	ID             ledger.ID
	Ledger         string
	Reference      string
	EventType      string
	Code           uint16
	BusinessTime   *time.Time
	Metadata       map[string]string
	IdempotencyKey string
	RequestID      string
	Reverts        *ledger.ID
	RevertedBy     *ledger.ID
	CommittedAt    time.Time
	TBTimestamp    uint64
	Legs           []LegRow
}

// TransactionFilter narrows the transaction listing.
type TransactionFilter struct {
	Address   ledger.Address
	Asset     string
	Reference string
	EventType string
	After     *time.Time
	Before    *time.Time
}

// HoldRow is a hold as Postgres stores it.
type HoldRow struct {
	ID                   ledger.ID
	Ledger               string
	PendingTransferID    ledger.ID
	Source               AccountRow
	Destination          AccountRow
	Reference            string
	EventType            string
	Code                 uint16
	Metadata             map[string]string
	TimeoutSeconds       uint32
	IdempotencyKey       string
	RequestID            string
	CreatedAt            time.Time
	TBTimestamp          uint64
	Status               HoldStatus
	ResolutionTransferID *ledger.ID
	ResolvedAt           *time.Time
}

// HoldResolution records a capture or void.
type HoldResolution struct {
	Ledger               string
	HoldID               ledger.ID
	Status               HoldStatus
	ResolutionTransferID ledger.ID
	ResolvedAt           time.Time
}

// HoldFilter narrows the hold listing.
type HoldFilter struct {
	Status  HoldStatus
	Address ledger.Address
}

// HoldCounts counts holds by visible status.
type HoldCounts struct {
	Open, Captured, Voided, Expired int64
}
