package readiness

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	t.Parallel()
	errDown := errors.New("down")
	up := func(context.Context) error { return nil }
	down := func(context.Context) error { return errDown }
	slow := func(ctx context.Context) error {
		select {
		case <-time.After(time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	tests := []struct {
		name               string
		postgres, tb       Probe
		wantReady          bool
		wantPGErr, wantTBE error
	}{
		{name: "both up", postgres: up, tb: up, wantReady: true},
		{name: "postgres down", postgres: down, tb: up, wantPGErr: errDown},
		{name: "a slow dependency is reported as a timeout", postgres: up, tb: slow, wantTBE: context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := New(20 * time.Millisecond)
			c.Require("postgres", tt.postgres)
			c.Require("tigerbeetle", tt.tb)
			report := c.Check(context.Background())
			if report.Ready() != tt.wantReady {
				t.Fatalf("Ready() = %v, want %v (report %+v)", report.Ready(), tt.wantReady, report)
			}
			pg, _ := report.Result("postgres")
			tb, _ := report.Result("tigerbeetle")
			if !errors.Is(pg.Err, tt.wantPGErr) || !errors.Is(tb.Err, tt.wantTBE) {
				t.Fatalf("errors = %v, %v; want %v, %v", pg.Err, tb.Err, tt.wantPGErr, tt.wantTBE)
			}
			if report[0].Name != "postgres" || report[1].Name != "tigerbeetle" {
				t.Fatalf("report order = %+v", report)
			}
		})
	}
	if _, ok := Report(nil).Result("missing"); ok {
		t.Fatal("Result found a probe that was never added")
	}
}

func TestDegraded(t *testing.T) {
	t.Parallel()
	down := func(context.Context) error { return errors.New("down") }
	up := func(context.Context) error { return nil }
	tests := []struct {
		name                string
		required, optional  Probe
		wantReady, wantDegr bool
	}{
		{name: "all up", required: up, optional: up, wantReady: true},
		{name: "optional down", required: up, optional: down, wantReady: true, wantDegr: true},
		{name: "required down", required: down, optional: down},
	}
	for _, tt := range tests {
		c := New(time.Second)
		c.Require("pg", tt.required)
		c.Optional("broker", tt.optional)
		r := c.Check(t.Context())
		if r.Ready() != tt.wantReady || r.Degraded() != tt.wantDegr {
			t.Errorf("%s: ready %v degraded %v", tt.name, r.Ready(), r.Degraded())
		}
	}
}
