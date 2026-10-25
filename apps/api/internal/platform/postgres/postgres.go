// Package postgres owns the Postgres connection pool of the process: opening it, readiness,
// migrations, the server and schema versions, transactions, and the mapping of driver errors to
// kernel errors. The modules build their stores on top of it; none of them opens a pool.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// UniqueViolation is the SQLSTATE Postgres raises when an INSERT hits a unique constraint.
const UniqueViolation = "23505"

// DB is the connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// Open parses the connection string and creates a pool. Connections are opened lazily; use Ready
// or Migrate to find out whether the database is reachable.
func Open(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", MapError(err))
	}
	return &DB{pool: pool}, nil
}

// Close drains the pool.
func (d *DB) Close() {
	d.pool.Close()
}

// Pool is the pgx pool the stores and sqlc's generated queries run on.
func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// Ready acquires a connection and pings the server.
func (d *DB) Ready(ctx context.Context) error {
	if err := d.pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: ping: %w", MapError(err))
	}
	return nil
}

// InTx runs fn in a Postgres transaction and commits when it returns nil.
func (d *DB) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op that returns ErrTxClosed.
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ServerVersion returns the Postgres server version, for the System page.
func (d *DB) ServerVersion(ctx context.Context) (string, error) {
	var v string
	if err := d.pool.QueryRow(ctx, "SHOW server_version").Scan(&v); err != nil {
		return "", fmt.Errorf("postgres: server version: %w", MapError(err))
	}
	return v, nil
}

// SchemaVersion returns the last migration goose applied.
func (d *DB) SchemaVersion(ctx context.Context) (int64, error) {
	var v int64
	if err := d.pool.QueryRow(ctx, "SELECT COALESCE(max(version_id), 0) FROM goose_db_version WHERE is_applied").Scan(&v); err != nil {
		return 0, fmt.Errorf("postgres: schema version: %w", MapError(err))
	}
	return v, nil
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

// MapError translates driver errors into kernel errors. The original error stays wrapped for logs;
// transports only ever show the kernel error to clients. A unique violation is ErrAlreadyExists,
// except on the named conflict constraints: unique indexes that encode "at most once" rules rather
// than identity, such as a second revert of one transaction, which are ErrConflict.
func MapError(err error, conflictConstraints ...string) error {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%w: %w", ledger.ErrNotFound, err)
	case errors.As(err, &pgErr) && pgErr.Code == UniqueViolation && slices.Contains(conflictConstraints, pgErr.ConstraintName):
		return fmt.Errorf("%w: %w", ledger.ErrConflict, err)
	case errors.As(err, &pgErr) && pgErr.Code == UniqueViolation:
		return fmt.Errorf("%w: %w", ledger.ErrAlreadyExists, err)
	case errors.As(err, &pgErr) && isOutageClass(pgErr.Code):
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	case errors.As(err, &pgErr):
		// Any other SQLSTATE means the server answered; it is a bug, not an outage.
		return err
	default:
		// Deadlines, cancellations and network failures: the server did not answer.
		return fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	}
}
