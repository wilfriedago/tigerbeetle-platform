package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

func TestMapError(t *testing.T) {
	t.Parallel()
	errOther := errors.New("dial tcp: connection refused")
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "no rows", err: pgx.ErrNoRows, want: ledger.ErrNotFound},
		{name: "unique violation", err: &pgconn.PgError{Code: uniqueViolation}, want: ledger.ErrAlreadyExists},
		{name: "timeout", err: context.DeadlineExceeded, want: ledger.ErrUnavailable},
		{name: "connection failure sqlstate", err: &pgconn.PgError{Code: "08006"}, want: ledger.ErrUnavailable},
		{name: "admin shutdown sqlstate", err: &pgconn.PgError{Code: "57P01"}, want: ledger.ErrUnavailable},
		{name: "network", err: errOther, want: ledger.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mapError(tt.err)
			if !errors.Is(got, tt.want) {
				t.Fatalf("mapError(%v) = %v, want %v", tt.err, got, tt.want)
			}
			if !errors.Is(got, tt.err) {
				t.Fatalf("mapError(%v) lost the original error", tt.err)
			}
		})
	}

	t.Run("other sql errors are not outages", func(t *testing.T) {
		t.Parallel()
		got := mapError(&pgconn.PgError{Code: "42P01"})
		if errors.Is(got, ledger.ErrUnavailable) || errors.Is(got, ledger.ErrNotFound) {
			t.Fatalf("mapError mapped an undefined-table error to a domain error: %v", got)
		}
	})
}
