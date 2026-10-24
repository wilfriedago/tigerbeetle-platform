// Package postgres implements the metadata port with pgx. Hand-written SQL lives in queries/ and
// migrations/; sqlc generates the typed accessors in gen/.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/domain/ledger"
)

// uniqueViolation is the SQLSTATE Postgres raises when an INSERT hits a unique constraint.
const uniqueViolation = "23505"

// Store is the Postgres adapter.
type Store struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// New parses the connection string and creates a pool. Connections are opened lazily; use Ready
// or Migrate to find out whether the database is reachable.
func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", mapError(err))
	}
	return &Store{pool: pool, queries: gen.New(pool)}, nil
}

// Close drains the pool.
func (s *Store) Close() {
	s.pool.Close()
}

// Ready acquires a connection and pings the server.
func (s *Store) Ready(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: ping: %w", mapError(err))
	}
	return nil
}

// CreateLedger inserts a ledger row.
func (s *Store) CreateLedger(ctx context.Context, l ledger.Ledger) (ledger.Ledger, error) {
	metadata, err := json.Marshal(l.Metadata)
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("postgres: encode metadata: %w", err)
	}
	row, err := s.queries.CreateLedger(ctx, gen.CreateLedgerParams{
		Name:      l.Name,
		Metadata:  metadata,
		CreatedAt: pgtype.Timestamptz{Time: l.CreatedAt, Valid: true},
	})
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("postgres: create ledger: %w", mapError(err))
	}
	return toLedger(row)
}

// GetLedger loads a ledger row by name.
func (s *Store) GetLedger(ctx context.Context, name string) (ledger.Ledger, error) {
	row, err := s.queries.GetLedger(ctx, name)
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("postgres: get ledger: %w", mapError(err))
	}
	return toLedger(row)
}

func toLedger(row gen.Ledger) (ledger.Ledger, error) {
	metadata := map[string]string{}
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
			return ledger.Ledger{}, fmt.Errorf("postgres: decode metadata of %q: %w", row.Name, err)
		}
	}
	return ledger.Ledger{Name: row.Name, Metadata: metadata, CreatedAt: row.CreatedAt.Time.UTC()}, nil
}

// mapError translates driver errors into domain errors. The original error stays wrapped for
// logs; the HTTP adapter only ever shows the domain error to clients.
func mapError(err error) error {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%w: %w", ledger.ErrNotFound, err)
	case errors.As(err, &pgErr) && pgErr.Code == uniqueViolation:
		return fmt.Errorf("%w: %w", ledger.ErrAlreadyExists, err)
	case errors.As(err, &pgErr):
		// Any other SQLSTATE means the server answered; it is a bug, not an outage.
		return err
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	default:
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	}
}
