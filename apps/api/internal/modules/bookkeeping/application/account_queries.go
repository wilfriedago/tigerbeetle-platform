package application

import (
	"context"
	"fmt"
	"math/big"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// LookupChunk is the most IDs one lookup_accounts or lookup_transfers request carries, the default
// TigerBeetle batch limit.
const LookupChunk = 8189

// MaxAggregateAccounts caps a prefix sum. Above it the request is rejected rather than answered
// slowly (ADR-0004); clients that need exact totals keep a dedicated aggregate account.
const MaxAggregateAccounts = 100_000

type accountQueries struct {
	ledgers  LedgerDirectory
	accounts domain.AccountRepository
	holds    domain.HoldRepository
	tb       domain.LedgerStore
	clock    ledger.Clock
}

func balanceView(row domain.AccountRow, state domain.AccountState) BalanceView {
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

// lookupStates reads accounts from TigerBeetle in chunks and indexes them by ID.
func lookupStates(ctx context.Context, tb domain.LedgerStore, ids []ledger.ID) (states map[ledger.ID]domain.AccountState, chunks int, err error) {
	states = make(map[ledger.ID]domain.AccountState, len(ids))
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

func (u *accountQueries) views(ctx context.Context, rows []domain.AddressRow) ([]AccountView, error) {
	var ids []ledger.ID
	for _, r := range rows {
		for i := range r.Accounts {
			ids = append(ids, r.Accounts[i].ID)
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

func (u *accountQueries) List(ctx context.Context, in ListAccountsInput) ([]AccountView, string, error) {
	if err := u.ledgers.Require(ctx, in.Ledger); err != nil {
		return nil, "", err
	}
	filter := domain.AddressFilter{Search: in.Search}
	p, err := parsePrefix(in.Prefix)
	if err != nil {
		return nil, "", err
	}
	filter.Prefix = p
	if in.Asset != "" {
		a, err := ledger.ParseAsset(in.Asset)
		if err != nil {
			return nil, "", err
		}
		filter.Asset = a.String()
	}
	after, err := ledger.DecodeCursor(in.Cursor)
	if err != nil {
		return nil, "", err
	}
	limit := ledger.PageSize(in.Limit)
	rows, err := u.accounts.ListAddresses(ctx, in.Ledger, filter, after, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list addresses: %w", err)
	}
	page, next := ledger.TrimPage(rows, limit, func(r domain.AddressRow) string { return string(r.Address) })
	views, err := u.views(ctx, page)
	if err != nil {
		return nil, "", err
	}
	return views, next, nil
}

func (u *accountQueries) Get(ctx context.Context, ledgerName, address string) (AccountView, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
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
	views, err := u.views(ctx, []domain.AddressRow{row})
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

func (u *accountQueries) History(ctx context.Context, ledgerName, address, asset string, limit int) ([]HistoryPoint, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
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
	rows, err := u.accounts.FindAccounts(ctx, ledgerName, []domain.AccountKey{{Address: a, Asset: as.String()}})
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

func (u *accountQueries) AggregateBalances(ctx context.Context, ledgerName, prefix string) (Aggregate, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
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
		ids[i] = rows[i].ID
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

func (u *accountQueries) Segments(ctx context.Context, ledgerName, prefix string) (string, []domain.SegmentRow, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
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

// Assets lists the assets of a ledger. Money enters and leaves through world (ADR-0006), so minus
// its balance is the cheapest "how much money is in this ledger" figure.
func (u *accountQueries) Assets(ctx context.Context, ledgerName string) ([]AssetView, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
		return nil, err
	}
	assets, err := u.accounts.ListAssets(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	keys := make([]domain.AccountKey, len(assets))
	for i, a := range assets {
		keys[i] = domain.AccountKey{Address: ledger.World, Asset: a.Asset}
	}
	worlds, err := u.accounts.FindAccounts(ctx, ledgerName, keys)
	if err != nil {
		return nil, fmt.Errorf("find world accounts: %w", err)
	}
	ids := make([]ledger.ID, len(worlds))
	worldByAsset := make(map[string]domain.AccountRow, len(worlds))
	for i := range worlds {
		ids[i] = worlds[i].ID
		worldByAsset[worlds[i].Asset] = worlds[i]
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
