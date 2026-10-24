package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/migrations"
)

// Migrate applies every pending goose migration embedded in the binary. goose is used as a
// library through a Provider instead of its global functions, so nothing is configured through
// package state.
func (s *Store) Migrate(ctx context.Context, logger *slog.Logger) error {
	// goose speaks database/sql; this bridge borrows connections from the pool and closing it
	// does not close the pool.
	db := stdlib.OpenDBFromPool(s.pool)
	defer func() { _ = db.Close() }()

	// A Postgres advisory lock serialises replicas that start at the same time with
	// MIGRATE_ON_START, so only one applies a migration and the others wait.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("postgres: migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("postgres: migrations: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("postgres: migrate up: %w", mapError(err))
	}
	for _, r := range results {
		logger.InfoContext(ctx, "migration applied",
			slog.String("migration", r.Source.Path),
			slog.Int64("version", r.Source.Version),
			slog.Duration("duration", r.Duration))
	}
	return nil
}
