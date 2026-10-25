package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// fakeStore keeps the bookkeeping tables in memory: enough of them for the write path. Methods the
// tests never reach come from the nil embedded interfaces and would panic.
type fakeStore struct {
	domain.AccountRepository
	domain.HoldRepository

	mu       sync.Mutex
	assets   map[string]domain.AssetRow
	accounts map[domain.AccountKey]domain.AccountRow
	writes   map[string]domain.WriteRecord
	txs      map[ledger.ID]domain.TransactionRow
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		assets: map[string]domain.AssetRow{}, accounts: map[domain.AccountKey]domain.AccountRow{},
		writes: map[string]domain.WriteRecord{}, txs: map[ledger.ID]domain.TransactionRow{},
	}
}

func (f *fakeStore) EnsureAssets(_ context.Context, _ string, assets []ledger.Asset) (map[string]domain.AssetRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]domain.AssetRow{}
	for _, a := range assets {
		row, ok := f.assets[a.String()]
		if !ok {
			row = domain.AssetRow{Asset: a.String(), TBLedger: uint32(len(f.assets) + 1)}
			f.assets[a.String()] = row
		}
		out[a.String()] = row
	}
	return out, nil
}

func (f *fakeStore) FindAccounts(_ context.Context, _ string, keys []domain.AccountKey) ([]domain.AccountRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.AccountRow
	for _, k := range keys {
		if row, ok := f.accounts[k]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeStore) EnsureAccounts(ctx context.Context, ledgerName string, candidates []domain.AccountRow) ([]domain.AccountRow, error) {
	f.mu.Lock()
	keys := make([]domain.AccountKey, len(candidates))
	for i := range candidates {
		c := &candidates[i]
		if _, ok := f.accounts[c.Key()]; !ok {
			f.accounts[c.Key()] = *c
		}
		keys[i] = c.Key()
	}
	f.mu.Unlock()
	return f.FindAccounts(ctx, ledgerName, keys)
}

func (f *fakeStore) BeginWrite(_ context.Context, rec domain.WriteRecord) (domain.WriteRecord, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if existing, ok := f.writes[rec.Key]; ok {
		return existing, false, nil
	}
	f.writes[rec.Key] = rec
	return rec, true, nil
}

func (f *fakeStore) GetWrite(_ context.Context, _, key string) (domain.WriteRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.writes[key]
	if !ok {
		return domain.WriteRecord{}, ledger.ErrNotFound
	}
	return rec, nil
}

func (*fakeStore) ListWrites(context.Context, string, domain.WriteFilter, domain.WriteCursor, int) ([]domain.WriteRecord, error) {
	return nil, errors.New("not used")
}

func (f *fakeStore) NoteAttempt(_ context.Context, _, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec := f.writes[key]
	rec.Attempts++
	f.writes[key] = rec
	return nil
}

func (f *fakeStore) resolve(o domain.WriteOutcome) {
	rec := f.writes[o.Key]
	rec.Status, rec.ResponseStatus, rec.Response, rec.ProblemType = o.Status, o.ResponseStatus, o.Response, o.ProblemType
	f.writes[o.Key] = rec
}

func (f *fakeStore) ResolveWrite(_ context.Context, _, _ string, o domain.WriteOutcome) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolve(o)
	return nil
}

func (f *fakeStore) CommitTransaction(_ context.Context, o domain.WriteOutcome, tx domain.TransactionRow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolve(o)
	f.txs[tx.ID] = tx
	return nil
}

func (*fakeStore) CommitHold(context.Context, domain.WriteOutcome, domain.HoldRow) error {
	return errors.New("not used")
}

func (*fakeStore) CommitResolution(context.Context, domain.WriteOutcome, domain.HoldResolution) error {
	return errors.New("not used")
}

func (f *fakeStore) PendingWrites(_ context.Context, before time.Time, _ int) ([]domain.WriteRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.WriteRecord
	for key := range f.writes {
		if rec := f.writes[key]; rec.Status == domain.RequestPending && rec.CreatedAt.Before(before) {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (*fakeStore) CountPending(context.Context, time.Time) (pending, stuck int64, err error) {
	return 0, 0, nil
}

func (*fakeStore) DeclinedSince(context.Context, string, time.Time) ([]domain.ReasonCount, error) {
	return nil, errors.New("not used")
}

func (f *fakeStore) GetTransaction(_ context.Context, _ string, id ledger.ID) (domain.TransactionRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tx, ok := f.txs[id]
	if !ok {
		return domain.TransactionRow{}, ledger.ErrNotFound
	}
	return tx, nil
}

func (*fakeStore) ListTransactions(context.Context, string, domain.TransactionFilter, ledger.ID, int) ([]domain.TransactionRow, error) {
	return nil, errors.New("not used")
}

func (*fakeStore) ReplaceTransactionMetadata(context.Context, string, ledger.ID, map[string]string) error {
	return errors.New("not used")
}

func (*fakeStore) SearchTransactions(context.Context, string, string, int) ([]domain.TransactionRow, error) {
	return nil, errors.New("not used")
}

func (*fakeStore) EventTypeUsage(context.Context, string) (map[string]int64, error) {
	return nil, errors.New("not used")
}

// fakeTigerBeetle keeps accounts and transfers, answers each transfer with result(transfer), and
// can fail the next request before it reaches the cluster.
type fakeTigerBeetle struct {
	mu        sync.Mutex
	accounts  map[ledger.ID]domain.AccountSpec
	transfers map[ledger.ID]domain.TransferRecord
	result    func(domain.TransferSpec) string
	failNext  bool
	now       uint64
}

func newFakeTigerBeetle() *fakeTigerBeetle {
	return &fakeTigerBeetle{
		accounts: map[ledger.ID]domain.AccountSpec{}, transfers: map[ledger.ID]domain.TransferRecord{},
		result: func(domain.TransferSpec) string { return domain.ResultCreated }, now: 1_000,
	}
}

func (*fakeTigerBeetle) Ready(context.Context) error { return nil }

func (f *fakeTigerBeetle) CreateAccounts(_ context.Context, specs []domain.AccountSpec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range specs {
		f.accounts[s.ID] = s
	}
	return nil
}

func (f *fakeTigerBeetle) CreateTransfers(_ context.Context, specs []domain.TransferSpec) ([]domain.TransferOutcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext {
		f.failNext = false
		return nil, ledger.ErrUnavailable
	}
	out := make([]domain.TransferOutcome, len(specs))
	declined := false
	for i := range specs {
		if r := f.result(specs[i]); r != domain.ResultCreated {
			out[i].Result, declined = r, true
		}
	}
	for i := range specs {
		s := &specs[i]
		if declined {
			if out[i].Result == "" {
				out[i].Result = domain.ResultLinkedEventFailed
			}
			continue
		}
		f.now++
		f.transfers[s.ID] = domain.TransferRecord{ID: s.ID, Amount: s.Amount, Timestamp: f.now}
		out[i] = domain.TransferOutcome{Result: domain.ResultCreated, Timestamp: f.now}
	}
	return out, nil
}

func (*fakeTigerBeetle) LookupAccounts(context.Context, []ledger.ID) ([]domain.AccountState, error) {
	return nil, nil
}

func (f *fakeTigerBeetle) LookupTransfers(_ context.Context, ids []ledger.ID) ([]domain.TransferRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.TransferRecord
	for _, id := range ids {
		if t, ok := f.transfers[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func (*fakeTigerBeetle) AccountBalances(context.Context, ledger.ID, int) ([]domain.BalancePoint, error) {
	return nil, nil
}

// counterIDs mints increasing IDs.
type counterIDs struct {
	mu sync.Mutex
	n  byte
}

func (c *counterIDs) NewID() ledger.ID {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return ledger.ID{15: c.n}
}

// fakeLedgers answers the ports the ledgers module serves: one ledger, "eu", the default rules and
// the default event type.
type fakeLedgers struct{}

func (fakeLedgers) Require(_ context.Context, name string) error {
	if name != "eu" {
		return ledger.ErrNotFound
	}
	return nil
}

func (fakeLedgers) PoliciesFor(_ context.Context, _ string, addresses []ledger.Address) (map[ledger.Address]ledger.Rule, error) {
	out := map[ledger.Address]ledger.Rule{}
	for _, a := range addresses {
		out[a], _ = ledger.MatchRule(nil, a)
	}
	return out, nil
}

func (fakeLedgers) Resolve(_ context.Context, _, name string) (ledger.EventType, error) {
	if name == "" || name == ledger.DefaultEventType.Name {
		return ledger.DefaultEventType, nil
	}
	return ledger.EventType{}, ledger.NewProblem(ledger.ErrInvalid, "unknown-event-type", "unknown")
}
