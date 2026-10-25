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
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// uniqueViolation is the SQLSTATE Postgres raises when an INSERT hits a unique constraint.
const uniqueViolation = "23505"

// conflictConstraints are unique indexes that encode "at most once" rules rather than identity: a
// second revert of a transaction, a second resolution of a hold. Hitting them is a conflict with
// the current state, not a duplicate create.
var conflictConstraints = map[string]bool{
	"write_requests_one_revert":     true,
	"write_requests_one_resolution": true,
	"transactions_reverts_key":      true,
}

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

// inTx runs fn in a Postgres transaction and commits when it returns nil.
func (s *Store) inTx(ctx context.Context, fn func(q *gen.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op that returns ErrTxClosed.
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ServerVersion returns the Postgres server version, for the System page.
func (s *Store) ServerVersion(ctx context.Context) (string, error) {
	var v string
	if err := s.pool.QueryRow(ctx, "SHOW server_version").Scan(&v); err != nil {
		return "", fmt.Errorf("postgres: server version: %w", mapError(err))
	}
	return v, nil
}

// SchemaVersion returns the last migration goose applied.
func (s *Store) SchemaVersion(ctx context.Context) (int64, error) {
	var v int64
	if err := s.pool.QueryRow(ctx, "SELECT COALESCE(max(version_id), 0) FROM goose_db_version WHERE is_applied").Scan(&v); err != nil {
		return 0, fmt.Errorf("postgres: schema version: %w", mapError(err))
	}
	return v, nil
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

// isOutageClass reports whether a SQLSTATE belongs to a class that means the server could not
// serve us rather than that our SQL was wrong: 08 connection exception, 53 insufficient
// resources, 57 operator intervention (shutdown, crash), 58 system error.
func isOutageClass(code string) bool {
	if len(code) < 2 {
		return false
	}
	switch code[:2] {
	case "08", "53", "57", "58":
		return true
	default:
		return false
	}
}

// mapError translates driver errors into domain errors. The original error stays wrapped for
// logs; the HTTP adapter only ever shows the domain error to clients.
func mapError(err error) error {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%w: %w", ledger.ErrNotFound, err)
	case errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && conflictConstraints[pgErr.ConstraintName]:
		return fmt.Errorf("%w: %w", ledger.ErrConflict, err)
	case errors.As(err, &pgErr) && pgErr.Code == uniqueViolation:
		return fmt.Errorf("%w: %w", ledger.ErrAlreadyExists, err)
	case errors.As(err, &pgErr) && isOutageClass(pgErr.Code):
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	case errors.As(err, &pgErr):
		// Any other SQLSTATE means the server answered; it is a bug, not an outage.
		return err
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	default:
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	}
}
