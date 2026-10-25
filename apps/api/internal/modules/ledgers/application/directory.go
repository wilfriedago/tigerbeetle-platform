package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/domain"
)

// Directory tells whether a ledger exists. It answers bookkeeping's LedgerDirectory port, and the
// settings use cases use it too.
type Directory struct {
	ledgers domain.LedgerRepository
}

// NewDirectory builds the directory. Bootstrap builds it before bookkeeping, which needs it.
func NewDirectory(ledgers domain.LedgerRepository) *Directory {
	return &Directory{ledgers: ledgers}
}

// Require returns an error matching ledger.ErrInvalid for a malformed name and ledger.ErrNotFound
// when the ledger does not exist.
func (d *Directory) Require(ctx context.Context, name string) error {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return err
	}
	if _, err := d.ledgers.GetLedger(ctx, name); err != nil {
		return fmt.Errorf("get ledger %q: %w", name, err)
	}
	return nil
}
