// Package postgres implements the bookkeeping module's repositories on the pool of
// internal/platform/postgres. Its queries, in queries/, touch only the tables the module owns:
// ledger_assets, addresses, accounts, write_requests, transactions, transaction_postings and
// holds (plan section 4.7); sqlc generates gen/ from them.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres/gen"
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

// Store implements domain.AccountRepository, domain.WriteRepository,
// domain.TransactionRepository and domain.HoldRepository.
type Store struct {
	db      *platformpg.DB
	queries *gen.Queries
}

// NewStore builds the store on the process's pool.
func NewStore(db *platformpg.DB) *Store {
	return &Store{db: db, queries: gen.New(db.Pool())}
}

// inTx runs fn in a Postgres transaction and commits when it returns nil.
func (s *Store) inTx(ctx context.Context, fn func(q *gen.Queries) error) error {
	return s.db.InTx(ctx, func(tx pgx.Tx) error { return fn(s.queries.WithTx(tx)) })
}

// mapError translates driver errors into kernel errors, with this store's conflict constraints.
func mapError(err error) error {
	return platformpg.MapError(err, conflictConstraints...)
}
