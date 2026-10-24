package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

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

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
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
