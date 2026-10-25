package application

import (
	"context"
	"fmt"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// accountEnsurer creates accounts lazily, on the first write that names them (ADR-0005, ADR-0006).
type accountEnsurer struct {
	accounts domain.AccountRepository
	policies AccountPolicies
	tb       domain.LedgerStore
	ids      domain.IDGenerator
	clock    ledger.Clock
}

// ensure resolves every (address, asset) pair to its TigerBeetle account, creating the missing ones
// from the rule the ledgers module says wins (ADR-0006). It allocates ledger numbers on first use
// (ADR-0005), inserts candidate rows with ON CONFLICT DO NOTHING, and sends every row, new or not,
// to create_accounts, which heals a crash between the Postgres insert and TigerBeetle.
func (e *accountEnsurer) ensure(ctx context.Context, ledgerName string, keys []domain.AccountKey) (map[domain.AccountKey]domain.AccountRow, error) {
	keys = uniqueKeys(keys)
	assets := make([]ledger.Asset, 0, len(keys))
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k.Asset] {
			continue
		}
		seen[k.Asset] = true
		a, err := ledger.ParseAsset(k.Asset)
		if err != nil {
			return nil, err
		}
		assets = append(assets, a)
	}
	assetRows, err := e.accounts.EnsureAssets(ctx, ledgerName, assets)
	if err != nil {
		return nil, fmt.Errorf("ensure assets: %w", err)
	}
	existing, err := e.accounts.FindAccounts(ctx, ledgerName, keys)
	if err != nil {
		return nil, fmt.Errorf("find accounts: %w", err)
	}
	byKey := make(map[domain.AccountKey]domain.AccountRow, len(keys))
	for i := range existing {
		byKey[existing[i].Key()] = existing[i]
	}
	var missing []ledger.Address
	missingSeen := map[ledger.Address]bool{}
	for _, k := range keys {
		if _, ok := byKey[k]; !ok && !missingSeen[k.Address] {
			missingSeen[k.Address] = true
			missing = append(missing, k.Address)
		}
	}
	if len(missing) > 0 {
		if err := e.create(ctx, ledgerName, keys, missing, assetRows, byKey); err != nil {
			return nil, err
		}
	}
	specs := make([]domain.AccountSpec, 0, len(keys))
	for _, k := range keys {
		r, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("account %s %s missing after insert", k.Address, k.Asset)
		}
		specs = append(specs, domain.AccountSpec{ID: r.ID, TBLedger: r.TBLedger, Code: r.Code, Limit: r.Limit, History: r.History})
	}
	if err := e.tb.CreateAccounts(ctx, specs); err != nil {
		return nil, fmt.Errorf("create accounts: %w", err)
	}
	return byKey, nil
}

// create inserts candidate rows for the pairs not in byKey and adds the rows that won to it.
func (e *accountEnsurer) create(ctx context.Context, ledgerName string, keys []domain.AccountKey, missing []ledger.Address,
	assetRows map[string]domain.AssetRow, byKey map[domain.AccountKey]domain.AccountRow,
) error {
	policies, err := e.policies.PoliciesFor(ctx, ledgerName, missing)
	if err != nil {
		return fmt.Errorf("account rules: %w", err)
	}
	now := e.clock.Now().UTC()
	var candidates []domain.AccountRow
	for _, k := range keys {
		if _, ok := byKey[k]; ok {
			continue
		}
		rule := policies[k.Address]
		candidates = append(candidates, domain.AccountRow{
			ID:          e.ids.NewID(),
			Ledger:      ledgerName,
			Address:     k.Address,
			Asset:       k.Asset,
			TBLedger:    assetRows[k.Asset].TBLedger,
			Code:        rule.Code,
			Limit:       rule.Limit,
			History:     rule.History,
			RulePattern: string(rule.Pattern),
			CreatedAt:   now,
		})
	}
	won, err := e.accounts.EnsureAccounts(ctx, ledgerName, candidates)
	if err != nil {
		return fmt.Errorf("ensure accounts: %w", err)
	}
	for i := range won {
		byKey[won[i].Key()] = won[i]
	}
	return nil
}

func uniqueKeys(keys []domain.AccountKey) []domain.AccountKey {
	seen := make(map[domain.AccountKey]bool, len(keys))
	out := make([]domain.AccountKey, 0, len(keys))
	for _, k := range keys {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}
