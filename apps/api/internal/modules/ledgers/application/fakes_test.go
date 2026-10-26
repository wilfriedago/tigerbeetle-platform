package application

import (
	"context"
	"slices"
	"strings"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// fakeStore keeps ledgers, rules and event types in memory.
type fakeStore struct {
	ledgers map[string]ledger.Ledger
	rules   map[string][]ledger.Rule
	events  map[string][]ledger.EventType
}

func newFakeStore() *fakeStore {
	return &fakeStore{ledgers: map[string]ledger.Ledger{}, rules: map[string][]ledger.Rule{}, events: map[string][]ledger.EventType{}}
}

func (f *fakeStore) CreateLedger(_ context.Context, l ledger.Ledger) (ledger.Ledger, error) {
	if _, ok := f.ledgers[l.Name]; ok {
		return ledger.Ledger{}, ledger.ErrAlreadyExists
	}
	f.ledgers[l.Name] = l
	return l, nil
}

func (f *fakeStore) GetLedger(_ context.Context, name string) (ledger.Ledger, error) {
	l, ok := f.ledgers[name]
	if !ok {
		return ledger.Ledger{}, ledger.ErrNotFound
	}
	return l, nil
}

func (f *fakeStore) ListLedgers(_ context.Context, all bool, names []string, after string, limit int) ([]ledger.Ledger, error) {
	var out []ledger.Ledger
	for _, l := range f.ledgers {
		if l.Name > after && (all || slices.Contains(names, l.Name)) {
			out = append(out, l)
		}
	}
	slices.SortFunc(out, func(a, b ledger.Ledger) int { return strings.Compare(a.Name, b.Name) })
	return out[:min(limit, len(out))], nil
}

func (f *fakeStore) ReplaceLedgerMetadata(_ context.Context, name string, metadata map[string]string) error {
	l, ok := f.ledgers[name]
	if !ok {
		return ledger.ErrNotFound
	}
	l.Metadata = metadata
	f.ledgers[name] = l
	return nil
}

func (f *fakeStore) ListRules(_ context.Context, ledgerName string) ([]ledger.Rule, error) {
	return f.rules[ledgerName], nil
}

func (f *fakeStore) PutRule(_ context.Context, ledgerName string, rule ledger.Rule) (ledger.Rule, error) {
	f.rules[ledgerName] = append(f.rules[ledgerName], rule)
	return rule, nil
}

func (f *fakeStore) DeleteRule(_ context.Context, ledgerName string, pattern ledger.Pattern) error {
	rules := f.rules[ledgerName]
	i := slices.IndexFunc(rules, func(r ledger.Rule) bool { return r.Pattern == pattern })
	if i < 0 {
		return ledger.ErrNotFound
	}
	f.rules[ledgerName] = slices.Delete(rules, i, i+1)
	return nil
}

func (f *fakeStore) ListEventTypes(_ context.Context, ledgerName string) ([]ledger.EventType, error) {
	return f.events[ledgerName], nil
}

func (f *fakeStore) GetEventType(_ context.Context, ledgerName, name string) (ledger.EventType, error) {
	for _, e := range f.events[ledgerName] {
		if e.Name == name {
			return e, nil
		}
	}
	return ledger.EventType{}, ledger.ErrNotFound
}

func (f *fakeStore) CreateEventType(_ context.Context, ledgerName, name string, code uint16) (ledger.EventType, error) {
	if code == 0 {
		code = uint16(len(f.events[ledgerName]) + 2)
	}
	e := ledger.EventType{Name: name, Code: code}
	f.events[ledgerName] = append(f.events[ledgerName], e)
	return e, nil
}

// fakeBookkeeping answers the ports the bookkeeping module serves.
type fakeBookkeeping struct {
	footprints map[string]ledger.Footprint
	usage      map[string]int64
	assets     map[ledger.Address][]string
}

func (f fakeBookkeeping) Footprints(_ context.Context, names []string) (map[string]ledger.Footprint, error) {
	out := map[string]ledger.Footprint{}
	for _, n := range names {
		if fp, ok := f.footprints[n]; ok {
			out[n] = fp
		}
	}
	return out, nil
}

func (f fakeBookkeeping) EventTypeUsage(context.Context, string) (map[string]int64, error) {
	return f.usage, nil
}

func (f fakeBookkeeping) AddressAssets(_ context.Context, _ string, a ledger.Address) ([]string, error) {
	return f.assets[a], nil
}

func newApplication(store *fakeStore, bk fakeBookkeeping, clock ledger.Clock) Application {
	return New(Deps{
		Ledgers: store, Rules: store, EventTypes: store, Directory: NewDirectory(store),
		Footprints: bk, Usage: bk, Addresses: bk, Clock: clock, Authorizer: allowAll{},
	})
}
