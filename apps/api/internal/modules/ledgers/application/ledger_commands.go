package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/domain"
)

type ledgerCommands struct {
	ledgers domain.LedgerRepository
	queries *ledgerQueries
	clock   ledger.Clock
}

func (u *ledgerCommands) Create(ctx context.Context, name string, metadata map[string]string) (LedgerView, error) {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return LedgerView{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return LedgerView{}, err
	}
	created, err := u.ledgers.CreateLedger(ctx, ledger.Ledger{Name: name, Metadata: metadata, CreatedAt: u.clock.Now().UTC()})
	if err != nil {
		return LedgerView{}, fmt.Errorf("create ledger %q: %w", name, err)
	}
	// A new ledger has no account yet.
	return LedgerView{Ledger: created, Footprint: ledger.Footprint{Assets: []string{}}}, nil
}

func (u *ledgerCommands) ReplaceMetadata(ctx context.Context, name string, metadata map[string]string) (LedgerView, error) {
	if err := ledger.ValidateLedgerName(name); err != nil {
		return LedgerView{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return LedgerView{}, err
	}
	if err := u.ledgers.ReplaceLedgerMetadata(ctx, name, metadata); err != nil {
		return LedgerView{}, fmt.Errorf("replace metadata of ledger %q: %w", name, err)
	}
	return u.queries.Get(ctx, name)
}
