package postgres

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// LedgerFootprints returns the assets and address count of each named ledger.
func (s *Store) LedgerFootprints(ctx context.Context, ledgerNames []string) (map[string]ledger.Footprint, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.LedgerFootprints(ctx, gen.LedgerFootprintsParams{OrganizationID: org, Ledgers: ledgerNames})
	if err != nil {
		return nil, fmt.Errorf("postgres: ledger footprints: %w", mapError(err))
	}
	out := make(map[string]ledger.Footprint, len(rows))
	for _, r := range rows {
		out[r.Ledger] = ledger.Footprint{Assets: r.Assets, Accounts: r.Accounts}
	}
	return out, nil
}

// EventTypeUsage counts transactions and holds per event type.
func (s *Store) EventTypeUsage(ctx context.Context, ledgerName string) (map[string]int64, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.EventTypeUsage(ctx, gen.EventTypeUsageParams{OrganizationID: org, Ledger: ledgerName})
	if err != nil {
		return nil, fmt.Errorf("postgres: event type usage: %w", mapError(err))
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.EventType] = r.Uses
	}
	return out, nil
}
