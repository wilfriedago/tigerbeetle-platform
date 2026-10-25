package postgres

import (
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

func accountRow(a gen.Account) domain.AccountRow {
	return domain.AccountRow{
		ID:          platformpg.IDOf(a.ID),
		Ledger:      a.Ledger,
		Address:     ledger.Address(a.Address),
		Asset:       a.Asset,
		TBLedger:    uint32(a.TbLedger),
		Code:        uint16(a.Code),
		Limit:       ledger.Limit(a.BalanceLimit),
		History:     a.History,
		RulePattern: a.RulePattern,
		CreatedAt:   platformpg.TimeOf(a.CreatedAt),
	}
}

func accountRows(as []gen.Account) []domain.AccountRow {
	out := make([]domain.AccountRow, len(as))
	for i := range as {
		a := &as[i]
		out[i] = accountRow(*a)
	}
	return out
}
