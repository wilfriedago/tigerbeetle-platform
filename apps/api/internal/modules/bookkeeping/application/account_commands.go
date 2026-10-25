package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

type accountCommands struct {
	ledgers  LedgerDirectory
	accounts domain.AccountRepository
	queries  *accountQueries
}

func (u *accountCommands) ReplaceMetadata(ctx context.Context, ledgerName, address string, metadata map[string]string) (AccountView, error) {
	if err := u.ledgers.Require(ctx, ledgerName); err != nil {
		return AccountView{}, err
	}
	a, err := ledger.ParseAddress(address)
	if err != nil {
		return AccountView{}, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	if err := ledger.ValidateMetadata(metadata); err != nil {
		return AccountView{}, err
	}
	if err := u.accounts.ReplaceAddressMetadata(ctx, ledgerName, a, metadata); err != nil {
		return AccountView{}, fmt.Errorf("replace metadata of %q: %w", a, err)
	}
	return u.queries.Get(ctx, ledgerName, address)
}
