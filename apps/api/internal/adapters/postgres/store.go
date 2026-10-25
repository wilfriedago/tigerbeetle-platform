// Package postgres implements the metadata port on the pool of internal/platform/postgres.
// Hand-written SQL lives in queries/ and migrations/; sqlc generates the typed accessors in gen/.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// conflictConstraints are unique indexes that encode "at most once" rules rather than identity: a
// second revert of a transaction, a second resolution of a hold. Hitting them is a conflict with
// the current state, not a duplicate create.
var conflictConstraints = []string{
	"write_requests_one_revert",
	"write_requests_one_resolution",
	"transactions_reverts_key",
}

// Store is the Postgres adapter.
type Store struct {
	db      *platformpg.DB
	queries *gen.Queries
}

// New builds the store on the process's pool.
func New(db *platformpg.DB) *Store {
	return &Store{db: db, queries: gen.New(db.Pool())}
}

// Ready pings the server.
func (s *Store) Ready(ctx context.Context) error {
	return s.db.Ready(ctx)
}

// inTx runs fn in a Postgres transaction and commits when it returns nil.
func (s *Store) inTx(ctx context.Context, fn func(q *gen.Queries) error) error {
	return s.db.InTx(ctx, func(tx pgx.Tx) error { return fn(s.queries.WithTx(tx)) })
}

// mapError translates driver errors into kernel errors, with this store's conflict constraints.
func mapError(err error) error {
	return platformpg.MapError(err, conflictConstraints...)
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
