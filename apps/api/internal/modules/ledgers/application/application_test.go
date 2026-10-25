package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

var now = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func clock() ledger.Clock { return ledger.ClockFunc(func() time.Time { return now }) }

func TestLedgers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("create, then get with the footprint bookkeeping reports", func(t *testing.T) {
		t.Parallel()
		bk := fakeBookkeeping{footprints: map[string]ledger.Footprint{"main": {Assets: []string{"EUR/2"}, Accounts: 3}}}
		app := newApplication(newFakeStore(), bk, clock())
		created, err := app.LedgerCommands.Create(ctx, "main", nil)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		want := LedgerView{
			Ledger:    ledger.Ledger{Name: "main", Metadata: map[string]string{}, CreatedAt: now},
			Footprint: ledger.Footprint{Assets: []string{}},
		}
		if diff := cmp.Diff(want, created); diff != "" {
			t.Fatalf("Create mismatch (-want +got):\n%s", diff)
		}
		got, err := app.LedgerQueries.Get(ctx, "main")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		want.Footprint = ledger.Footprint{Assets: []string{"EUR/2"}, Accounts: 3}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Get mismatch (-want +got):\n%s", diff)
		}
		updated, err := app.LedgerCommands.ReplaceMetadata(ctx, "main", map[string]string{"env": "test"})
		if err != nil || updated.Metadata["env"] != "test" || updated.Accounts != 3 {
			t.Fatalf("ReplaceMetadata = %+v, %v", updated, err)
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		store := newFakeStore()
		store.ledgers["taken"] = ledger.Ledger{Name: "taken"}
		app := newApplication(store, fakeBookkeeping{}, clock())
		tests := []struct {
			name string
			call func() error
			want error
		}{
			{name: "invalid name", call: func() error { _, err := app.LedgerCommands.Create(ctx, "Bad Name", nil); return err }, want: ledger.ErrInvalid},
			{name: "duplicate", call: func() error { _, err := app.LedgerCommands.Create(ctx, "taken", nil); return err }, want: ledger.ErrAlreadyExists},
			{name: "missing", call: func() error { _, err := app.LedgerQueries.Get(ctx, "missing"); return err }, want: ledger.ErrNotFound},
			{name: "invalid get", call: func() error { _, err := app.LedgerQueries.Get(ctx, ""); return err }, want: ledger.ErrInvalid},
			{name: "invalid metadata", call: func() error {
				_, err := app.LedgerCommands.Create(ctx, "fine", map[string]string{"bad key": "x"})
				return err
			}, want: ledger.ErrInvalid},
			{name: "metadata of a missing ledger", call: func() error {
				_, err := app.LedgerCommands.ReplaceMetadata(ctx, "missing", nil)
				return err
			}, want: ledger.ErrNotFound},
		}
		for _, tt := range tests {
			if err := tt.call(); !errors.Is(err, tt.want) {
				t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
			}
		}
	})
}

func TestLedgerPages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	app := newApplication(newFakeStore(), fakeBookkeeping{}, clock())
	for _, name := range []string{"c", "a", "b"} {
		if _, err := app.LedgerCommands.Create(ctx, name, nil); err != nil {
			t.Fatal(err)
		}
	}
	page, next, err := app.LedgerQueries.List(ctx, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Name != "a" || page[1].Name != "b" || next == "" || page[0].Assets == nil {
		t.Fatalf("first page = %+v next %q", page, next)
	}
	page, next, err = app.LedgerQueries.List(ctx, next, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].Name != "c" || next != "" {
		t.Fatalf("second page = %+v next %q", page, next)
	}
	if _, _, err := app.LedgerQueries.List(ctx, "not a cursor!", 2); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatalf("bad cursor error = %v", err)
	}
}

func TestSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newFakeStore()
	store.ledgers["eu"] = ledger.Ledger{Name: "eu"}
	bk := fakeBookkeeping{
		usage:  map[string]int64{"default": 4, "card.settled": 2},
		assets: map[ledger.Address][]string{"users:1:wallet": {"EUR/2"}},
	}
	app := newApplication(store, bk, clock())

	t.Run("rules", func(t *testing.T) {
		rule := ledger.Rule{Pattern: "users:", Limit: ledger.LimitDebitsMustNotExceedCredits, Code: 7}
		saved, err := app.SettingsCommands.PutRule(ctx, "eu", rule)
		if err != nil || saved.CreatedAt != now || saved.BuiltIn {
			t.Fatalf("PutRule = %+v, %v", saved, err)
		}
		match, err := app.SettingsQueries.MatchRule(ctx, "eu", "users:1:wallet")
		if err != nil || match.Rule.Pattern != "users:" || len(match.Existing) != 1 {
			t.Fatalf("MatchRule = %+v, %v", match, err)
		}
		if match, err := app.SettingsQueries.MatchRule(ctx, "eu", "merchants:9"); err != nil || match.Existing == nil {
			t.Fatalf("MatchRule of a new address = %+v, %v", match, err)
		}
		rules, err := app.SettingsQueries.Rules(ctx, "eu")
		if err != nil || len(rules) != 3 {
			t.Fatalf("Rules = %+v, %v", rules, err)
		}
		if err := app.SettingsCommands.DeleteRule(ctx, "eu", "users:"); err != nil {
			t.Fatalf("DeleteRule: %v", err)
		}
		var p *ledger.Problem
		if err := app.SettingsCommands.DeleteRule(ctx, "eu", "*"); !errors.As(err, &p) || p.Reason != "built-in-rule" {
			t.Fatalf("deleting the built-in default = %v", err)
		}
		if _, err := app.SettingsQueries.Rules(ctx, "missing"); !errors.Is(err, ledger.ErrNotFound) {
			t.Fatalf("rules of a missing ledger = %v", err)
		}
	})

	t.Run("event types", func(t *testing.T) {
		if _, err := app.SettingsCommands.CreateEventType(ctx, "eu", "card.settled", 9); err != nil {
			t.Fatal(err)
		}
		if _, err := app.SettingsCommands.CreateEventType(ctx, "eu", "card.authorized", 3); err != nil {
			t.Fatal(err)
		}
		var p *ledger.Problem
		if _, err := app.SettingsCommands.CreateEventType(ctx, "eu", "default", 0); !errors.As(err, &p) || !errors.Is(err, ledger.ErrAlreadyExists) {
			t.Fatalf("creating default = %v", err)
		}
		if _, err := app.SettingsCommands.CreateEventType(ctx, "eu", "x", 1); !errors.Is(err, ledger.ErrInvalid) {
			t.Fatalf("code 1 = %v", err)
		}
		types, err := app.SettingsQueries.EventTypes(ctx, "eu")
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(types))
		for i, e := range types {
			got[i] = e.Name
		}
		if diff := cmp.Diff([]string{"default", "card.authorized", "card.settled"}, got); diff != "" {
			t.Fatalf("order (-want +got):\n%s", diff)
		}
		if types[0].Uses != 4 || types[2].Uses != 2 || !types[0].BuiltIn {
			t.Fatalf("usage = %+v", types)
		}
	})
}

func TestPolicyQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newFakeStore()
	store.rules["eu"] = []ledger.Rule{{Pattern: "users:", Limit: ledger.LimitDebitsMustNotExceedCredits, Code: 7}}
	store.events["eu"] = []ledger.EventType{{Name: "card.settled", Code: 9}}
	q := NewPolicyQueries(store, store)

	policies, err := q.PoliciesFor(ctx, "eu", []ledger.Address{"users:1:wallet", "world", "fees"})
	if err != nil {
		t.Fatal(err)
	}
	if policies["users:1:wallet"].Code != 7 || policies["world"].Pattern != "world" || policies["fees"].Pattern != ledger.CatchAll {
		t.Fatalf("PoliciesFor = %+v", policies)
	}
	for name, want := range map[string]uint16{"": 1, "default": 1, "card.settled": 9} {
		e, err := q.Resolve(ctx, "eu", name)
		if err != nil || e.Code != want {
			t.Errorf("Resolve(%q) = %+v, %v", name, e, err)
		}
	}
	var p *ledger.Problem
	if _, err := q.Resolve(ctx, "eu", "missing"); !errors.As(err, &p) || p.Reason != "unknown-event-type" {
		t.Fatalf("Resolve of an unknown type = %v", err)
	}
	if err := NewDirectory(store).Require(ctx, "eu"); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("Require of a missing ledger = %v", err)
	}
}
