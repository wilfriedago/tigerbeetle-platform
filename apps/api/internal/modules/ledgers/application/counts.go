package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/domain"
)

// LedgerCounts answers the access module's LedgerCounter port: how many ledgers each organization
// has. It checks no permission itself; the access module calls it only for staff, who hold
// platform.organizations.
type LedgerCounts struct {
	counter domain.LedgerCounter
}

// NewLedgerCounts builds the counts on the store.
func NewLedgerCounts(counter domain.LedgerCounter) *LedgerCounts {
	return &LedgerCounts{counter: counter}
}

// CountLedgers counts the ledgers of each organization; an organization without one is absent.
func (c *LedgerCounts) CountLedgers(ctx context.Context, organizations []ledger.ID) (map[ledger.ID]int, error) {
	counts, err := c.counter.CountLedgers(ctx, organizations)
	if err != nil {
		return nil, fmt.Errorf("count ledgers: %w", err)
	}
	return counts, nil
}
