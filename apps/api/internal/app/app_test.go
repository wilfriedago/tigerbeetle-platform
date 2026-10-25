package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// fakeStore implements both store ports with programmable behaviour.
type fakeStore struct {
	readyErr   error
	readyDelay time.Duration
	ledgers    map[string]ledger.Ledger
}

func (f *fakeStore) Ready(ctx context.Context) error {
	if f.readyDelay > 0 {
		select {
		case <-time.After(f.readyDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.readyErr
}

func (f *fakeStore) CreateLedger(_ context.Context, l ledger.Ledger) (ledger.Ledger, error) {
	if _, ok := f.ledgers[l.Name]; ok {
		return ledger.Ledger{}, ledger.ErrAlreadyExists
	}
	if f.ledgers == nil {
		f.ledgers = map[string]ledger.Ledger{}
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

func (f *fakeStore) LedgerSummary(ctx context.Context, name string) (LedgerSummary, error) {
	l, err := f.GetLedger(ctx, name)
	if err != nil {
		return LedgerSummary{}, err
	}
	return LedgerSummary{Ledger: l, Assets: []string{}}, nil
}

func (f *fakeStore) ListLedgers(_ context.Context, after string, limit int) ([]LedgerSummary, error) {
	names := make([]string, 0, len(f.ledgers))
	for name := range f.ledgers {
		if name > after {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	out := []LedgerSummary{}
	for _, name := range names[:min(limit, len(names))] {
		out = append(out, LedgerSummary{Ledger: f.ledgers[name], Assets: []string{}})
	}
	return out, nil
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

func TestStatusCheck(t *testing.T) {
	t.Parallel()
	errDown := errors.New("down")
	tests := []struct {
		name        string
		postgres    *fakeStore
		tigerbeetle *fakeStore
		wantReady   bool
		wantPGErr   error
		wantTBErr   error
	}{
		{
			name:        "both up",
			postgres:    &fakeStore{},
			tigerbeetle: &fakeStore{},
			wantReady:   true,
		},
		{
			name:        "postgres down",
			postgres:    &fakeStore{readyErr: errDown},
			tigerbeetle: &fakeStore{},
			wantPGErr:   errDown,
		},
		{
			name:        "tigerbeetle slow is reported as a timeout",
			postgres:    &fakeStore{},
			tigerbeetle: &fakeStore{readyDelay: time.Second},
			wantTBErr:   context.DeadlineExceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status := NewStatus(tt.tigerbeetle, tt.postgres, 20*time.Millisecond)
			report := status.Check(context.Background())
			if report.Ready() != tt.wantReady {
				t.Fatalf("Ready() = %v, want %v (report %+v)", report.Ready(), tt.wantReady, report)
			}
			if !errors.Is(report.Postgres.Err, tt.wantPGErr) {
				t.Errorf("postgres err = %v, want %v", report.Postgres.Err, tt.wantPGErr)
			}
			if !errors.Is(report.TigerBeetle.Err, tt.wantTBErr) {
				t.Errorf("tigerbeetle err = %v, want %v", report.TigerBeetle.Err, tt.wantTBErr)
			}
		})
	}
}

func TestLedgers(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	clock := ClockFunc(func() time.Time { return now })

	t.Run("create then get", func(t *testing.T) {
		t.Parallel()
		u := NewLedgers(&fakeStore{}, clock)
		created, err := u.Create(context.Background(), "main", nil)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		want := ledger.Ledger{Name: "main", Metadata: map[string]string{}, CreatedAt: now}
		if diff := cmp.Diff(want, created.Ledger); diff != "" {
			t.Fatalf("Create mismatch (-want +got):\n%s", diff)
		}
		got, err := u.Get(context.Background(), "main")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Get mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		u := NewLedgers(&fakeStore{ledgers: map[string]ledger.Ledger{"taken": {Name: "taken"}}}, clock)
		tests := []struct {
			name string
			call func() error
			want error
		}{
			{name: "invalid name", call: func() error { _, err := u.Create(context.Background(), "Bad Name", nil); return err }, want: ledger.ErrInvalid},
			{name: "duplicate", call: func() error { _, err := u.Create(context.Background(), "taken", nil); return err }, want: ledger.ErrAlreadyExists},
			{name: "missing", call: func() error { _, err := u.Get(context.Background(), "missing"); return err }, want: ledger.ErrNotFound},
			{name: "invalid get", call: func() error { _, err := u.Get(context.Background(), ""); return err }, want: ledger.ErrInvalid},
			{name: "invalid metadata", call: func() error {
				_, err := u.Create(context.Background(), "fine", map[string]string{"bad key": "x"})
				return err
			}, want: ledger.ErrInvalid},
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
	store := &fakeStore{}
	u := NewLedgers(store, ClockFunc(time.Now))
	for _, name := range []string{"c", "a", "b"} {
		if _, err := u.Create(context.Background(), name, nil); err != nil {
			t.Fatal(err)
		}
	}
	page, next, err := u.List(context.Background(), "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Name != "a" || page[1].Name != "b" || next == "" {
		t.Fatalf("first page = %v next %q", page, next)
	}
	page, next, err = u.List(context.Background(), next, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].Name != "c" || next != "" {
		t.Fatalf("second page = %v next %q", page, next)
	}
	if _, _, err := u.List(context.Background(), "not a cursor!", 2); !errors.Is(err, ledger.ErrInvalid) {
		t.Fatalf("bad cursor error = %v", err)
	}
}
