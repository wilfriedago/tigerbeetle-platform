package app

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// LookupChunk is the most IDs one lookup_accounts or lookup_transfers request carries, the default
// TigerBeetle batch limit.
const LookupChunk = 8189

// MaxAggregateAccounts caps a prefix sum. Above it the request is rejected rather than answered
// slowly (ADR-0004); clients that need exact totals keep a dedicated aggregate account.
const MaxAggregateAccounts = 100_000

// BalanceView is one TigerBeetle account of an address with its balances read from the cluster.
type BalanceView struct {
	AccountRow
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

func balanceView(row AccountRow, state AccountState) BalanceView {
	balance := new(big.Int).Sub(state.CreditsPosted.Big(), state.DebitsPosted.Big())
	return BalanceView{
		AccountRow: row,
		Balance:    balance,
		Available:  new(big.Int).Sub(balance, state.DebitsPending.Big()),
		Reserved:   state.DebitsPending,
		Incoming:   state.CreditsPending,
		Credits:    state.CreditsPosted,
		Debits:     state.DebitsPosted,
	}
}

// AccountView is an address with one balance per asset.
type AccountView struct {
	Address   ledger.Address
	Metadata  map[string]string
	CreatedAt time.Time
	Balances  []BalanceView
	OpenHolds int64
}

// Accounts reads accounts and balances. Postgres resolves names to IDs; TigerBeetle answers every
// amount (ADR-0002).
type Accounts struct {
	ledgers  LedgerRepository
	accounts AccountRepository
	holds    HoldRepository
	tb       LedgerStore
	clock    Clock
}

// NewAccounts builds the use case.
func NewAccounts(ledgers LedgerRepository, accounts AccountRepository, holds HoldRepository, tb LedgerStore, clock Clock) *Accounts {
	return &Accounts{ledgers: ledgers, accounts: accounts, holds: holds, tb: tb, clock: clock}
}

func requireLedger(ctx context.Context, ledgers LedgerRepository, name string) error {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return err
	}
	if _, err := ledgers.GetLedger(ctx, name); err != nil {
		return fmt.Errorf("get ledger %q: %w", name, err)
	}
	return nil
}

// parsePrefix accepts "users", "users:" or "" and returns the address form without the colon.
func parsePrefix(prefix string) (ledger.Address, error) {
	trimmed := strings.TrimSuffix(prefix, ":")
	if trimmed == "" {
		return "", nil
	}
	return ledger.ParseAddress(trimmed)
}

// lookupStates reads accounts from TigerBeetle in chunks and indexes them by ID.
func lookupStates(ctx context.Context, tb LedgerStore, ids []ledger.ID) (states map[ledger.ID]AccountState, chunks int, err error) {
	states = make(map[ledger.ID]AccountState, len(ids))
	for start := 0; start < len(ids); start += LookupChunk {
		end := min(start+LookupChunk, len(ids))
		found, err := tb.LookupAccounts(ctx, ids[start:end])
		if err != nil {
			return nil, chunks, fmt.Errorf("lookup accounts: %w", err)
		}
		chunks++
		for _, s := range found {
			states[s.ID] = s
		}
	}
	return states, chunks, nil
}

func (u *Accounts) views(ctx context.Context, rows []AddressRow) ([]AccountView, error) {
	var ids []ledger.ID
	for _, r := range rows {
		for i := range r.Accounts {
			a := &r.Accounts[i]
			ids = append(ids, a.ID)
		}
	}
	states, _, err := lookupStates(ctx, u.tb, ids)
	if err != nil {
		return nil, err
	}
	views := make([]AccountView, 0, len(rows))
	for _, r := range rows {
		v := AccountView{Address: r.Address, Metadata: r.Metadata, CreatedAt: r.CreatedAt, Balances: make([]BalanceView, 0, len(r.Accounts))}
		for i := range r.Accounts {
			a := &r.Accounts[i]
			// An account row without its TigerBeetle account is the crash window of ADR-0006: the next
			// write that names the address creates it. Until then it has no balance.
			v.Balances = append(v.Balances, balanceView(*a, states[a.ID]))
		}
		views = append(views, v)
	}
	return views, nil
}

// List returns a page of accounts with their balances, in address order.
func (u *Accounts) List(ctx context.Context, ledgerName, prefix, search, asset, cursor string, limit int) ([]AccountView, string, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return nil, "", err
	}
	filter := AddressFilter{Search: search}
	p, err := parsePrefix(prefix)
	if err != nil {
		return nil, "", err
	}
	filter.Prefix = p
	if asset != "" {
		a, err := ledger.ParseAsset(asset)
		if err != nil {
			return nil, "", err
		}
		filter.Asset = a.String()
	}
	after, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	limit = pageSize(limit)
	rows, err := u.accounts.ListAddresses(ctx, ledgerName, filter, after, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list addresses: %w", err)
	}
	page, next := trimPage(rows, limit, func(r AddressRow) string { return string(r.Address) })
	views, err := u.views(ctx, page)
	if err != nil {
		return nil, "", err
	}
	return views, next, nil
}

// Get returns one account with its balances and open hold count.
func (u *Accounts) Get(ctx context.Context, ledgerName, address string) (AccountView, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return AccountView{}, err
	}
	a, err := ledger.ParseAddress(address)
	if err != nil {
		return AccountView{}, err
	}
	row, err := u.accounts.GetAddress(ctx, ledgerName, a)
	if err != nil {
		return AccountView{}, fmt.Errorf("get address %q: %w", a, err)
	}
	views, err := u.views(ctx, []AddressRow{row})
	if err != nil {
		return AccountView{}, err
	}
	v := views[0]
	v.OpenHolds, err = u.holds.CountOpenHoldsFrom(ctx, ledgerName, a, u.clock.Now())
	if err != nil {
		return AccountView{}, fmt.Errorf("count holds of %q: %w", a, err)
	}
	return v, nil
}

// ReplaceMetadata replaces the metadata of an existing address.
func (u *Accounts) ReplaceMetadata(ctx context.Context, ledgerName, address string, metadata map[string]string) (AccountView, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return AccountView{}, err
	}
	a, err := ledger.ParseAddress(address)
	if err != nil {
		return AccountView{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return AccountView{}, err
	}
	if err := u.accounts.ReplaceAddressMetadata(ctx, ledgerName, a, metadata); err != nil {
		return AccountView{}, fmt.Errorf("replace metadata of %q: %w", a, err)
	}
	return u.Get(ctx, ledgerName, address)
}

// HistoryPoint is one historical balance.
type HistoryPoint struct {
	Timestamp uint64
	Balance   *big.Int
	Reserved  ledger.Amount
	Credits   ledger.Amount
	Debits    ledger.Amount
}

// History returns point-in-time balances of the account for one asset. Only accounts created with
// the history flag have them, and the flag cannot be added later (ADR-0006).
func (u *Accounts) History(ctx context.Context, ledgerName, address, asset string, limit int) ([]HistoryPoint, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return nil, err
	}
	a, err := ledger.ParseAddress(address)
	if err != nil {
		return nil, err
	}
	as, err := ledger.ParseAsset(asset)
	if err != nil {
		return nil, err
	}
	rows, err := u.accounts.FindAccounts(ctx, ledgerName, []AccountKey{{Address: a, Asset: as.String()}})
	if err != nil {
		return nil, fmt.Errorf("find account: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %s has no %s account", ledger.ErrNotFound, a, as)
	}
	if !rows[0].History {
		return nil, ledger.NewProblem(ledger.ErrRejected, "history-not-enabled",
			fmt.Sprintf("%s in %s was created without the history flag by rule %s; the flag cannot be added to an existing account", a, as, rows[0].RulePattern))
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	points, err := u.tb.AccountBalances(ctx, rows[0].ID, limit)
	if err != nil {
		return nil, fmt.Errorf("account balances: %w", err)
	}
	out := make([]HistoryPoint, 0, len(points))
	for _, p := range points {
		out = append(out, HistoryPoint{
			Timestamp: p.Timestamp,
			Balance:   new(big.Int).Sub(p.CreditsPosted.Big(), p.DebitsPosted.Big()),
			Reserved:  p.DebitsPending,
			Credits:   p.CreditsPosted,
			Debits:    p.DebitsPosted,
		})
	}
	return out, nil
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

// AggregateBalances sums the balances of every account under a prefix, per asset. Postgres
// resolves the prefix to account IDs and TigerBeetle is read in chunks, so the result is not one
// consistent snapshot when it takes more than one chunk (ADR-0004).
func (u *Accounts) AggregateBalances(ctx context.Context, ledgerName, prefix string) (Aggregate, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return Aggregate{}, err
	}
	p, err := parsePrefix(prefix)
	if err != nil {
		return Aggregate{}, err
	}
	if p == "" {
		return Aggregate{}, fmt.Errorf("%w: address prefix is required", ledger.ErrInvalid)
	}
	rows, err := u.accounts.AccountsUnder(ctx, ledgerName, p, MaxAggregateAccounts+1)
	if err != nil {
		return Aggregate{}, fmt.Errorf("accounts under %q: %w", p, err)
	}
	if len(rows) > MaxAggregateAccounts {
		return Aggregate{}, ledger.NewProblem(ledger.ErrRejected, "prefix-too-broad",
			fmt.Sprintf("%s covers more than %d accounts; narrow the prefix. The limit protects TigerBeetle from oversized lookups", p, MaxAggregateAccounts))
	}
	ids := make([]ledger.ID, len(rows))
	for i := range rows {
		r := &rows[i]
		ids[i] = r.ID
	}
	states, chunks, err := lookupStates(ctx, u.tb, ids)
	if err != nil {
		return Aggregate{}, err
	}
	byAsset := map[string]*AggregateAsset{}
	order := []string{}
	addresses := map[ledger.Address]struct{}{}
	for k := range rows {
		r := &rows[k]
		addresses[r.Address] = struct{}{}
		sum, ok := byAsset[r.Asset]
		if !ok {
			sum = &AggregateAsset{Asset: r.Asset, Balance: new(big.Int), Available: new(big.Int), Reserved: new(big.Int), Incoming: new(big.Int)}
			byAsset[r.Asset] = sum
			order = append(order, r.Asset)
		}
		v := balanceView(*r, states[r.ID])
		sum.Accounts++
		sum.Balance.Add(sum.Balance, v.Balance)
		sum.Available.Add(sum.Available, v.Available)
		sum.Reserved.Add(sum.Reserved, v.Reserved.Big())
		sum.Incoming.Add(sum.Incoming, v.Incoming.Big())
	}
	out := Aggregate{Prefix: string(p) + ":", AsOf: u.clock.Now().UTC(), Addresses: int64(len(addresses)), Chunks: chunks}
	for _, asset := range order {
		out.Assets = append(out.Assets, *byAsset[asset])
	}
	return out, nil
}

// Segments returns the chart of accounts one level below a prefix.
func (u *Accounts) Segments(ctx context.Context, ledgerName, prefix string) (string, []SegmentRow, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return "", nil, err
	}
	p, err := parsePrefix(prefix)
	if err != nil {
		return "", nil, err
	}
	rows, err := u.accounts.Segments(ctx, ledgerName, p)
	if err != nil {
		return "", nil, fmt.Errorf("segments under %q: %w", p, err)
	}
	return string(p), rows, nil
}

// AssetView is an asset of a ledger with the money issued, minus the balance of world.
type AssetView struct {
	AssetRow
	Issued *big.Int
}

// Assets lists the assets of a ledger. Money enters and leaves through world (ADR-0006), so minus
// its balance is the cheapest "how much money is in this ledger" figure.
func (u *Accounts) Assets(ctx context.Context, ledgerName string) ([]AssetView, error) {
	if err := requireLedger(ctx, u.ledgers, ledgerName); err != nil {
		return nil, err
	}
	assets, err := u.accounts.ListAssets(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	keys := make([]AccountKey, len(assets))
	for i, a := range assets {
		keys[i] = AccountKey{Address: ledger.World, Asset: a.Asset}
	}
	worlds, err := u.accounts.FindAccounts(ctx, ledgerName, keys)
	if err != nil {
		return nil, fmt.Errorf("find world accounts: %w", err)
	}
	ids := make([]ledger.ID, len(worlds))
	worldByAsset := make(map[string]AccountRow, len(worlds))
	for i := range worlds {
		w := &worlds[i]
		ids[i] = w.ID
		worldByAsset[w.Asset] = *w
	}
	states, _, err := lookupStates(ctx, u.tb, ids)
	if err != nil {
		return nil, err
	}
	out := make([]AssetView, 0, len(assets))
	for _, a := range assets {
		issued := new(big.Int)
		if w, ok := worldByAsset[a.Asset]; ok {
			issued.Neg(balanceView(w, states[w.ID]).Balance)
		}
		out = append(out, AssetView{AssetRow: a, Issued: issued})
	}
	return out, nil
}
