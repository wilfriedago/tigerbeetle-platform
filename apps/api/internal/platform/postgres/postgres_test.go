package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
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
		{name: "unique violation", err: &pgconn.PgError{Code: UniqueViolation}, want: ledger.ErrAlreadyExists},
		{
			name: "unique violation on a conflict constraint",
			err:  &pgconn.PgError{Code: UniqueViolation, ConstraintName: "write_requests_one_revert"}, want: ledger.ErrConflict,
		},
		{name: "timeout", err: context.DeadlineExceeded, want: ledger.ErrUnavailable},
		{name: "connection failure sqlstate", err: &pgconn.PgError{Code: "08006"}, want: ledger.ErrUnavailable},
		{name: "admin shutdown sqlstate", err: &pgconn.PgError{Code: "57P01"}, want: ledger.ErrUnavailable},
		{name: "network", err: errOther, want: ledger.ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MapError(tt.err, "write_requests_one_revert")
			if !errors.Is(got, tt.want) {
				t.Fatalf("MapError(%v) = %v, want %v", tt.err, got, tt.want)
			}
			if !errors.Is(got, tt.err) {
				t.Fatalf("MapError(%v) lost the original error", tt.err)
			}
		})
	}

	t.Run("other sql errors are not outages", func(t *testing.T) {
		t.Parallel()
		got := MapError(&pgconn.PgError{Code: "42P01"})
		if errors.Is(got, ledger.ErrUnavailable) || errors.Is(got, ledger.ErrNotFound) {
			t.Fatalf("MapError mapped an undefined-table error to a domain error: %v", got)
		}
	})
}
