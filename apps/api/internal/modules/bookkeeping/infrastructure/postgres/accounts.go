package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// EnsureAssets allocates ledger numbers for new (ledger, asset) pairs with INSERT ... ON CONFLICT DO
// NOTHING and re-selects them, so concurrent first uses agree on one number (ADR-0005).
func (s *Store) EnsureAssets(ctx context.Context, ledgerName string, assets []ledger.Asset) (map[string]domain.AssetRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(assets))
	for i, a := range assets {
		names[i] = a.String()
	}
	if err := s.queries.EnsureAssets(ctx, gen.EnsureAssetsParams{OrganizationID: org, Ledger: ledgerName, Assets: names}); err != nil {
		return nil, fmt.Errorf("postgres: ensure assets: %w", mapError(err))
	}
	rows, err := s.queries.SelectAssets(ctx, gen.SelectAssetsParams{OrganizationID: org, Ledger: ledgerName, Assets: names})
	if err != nil {
		return nil, fmt.Errorf("postgres: select assets: %w", mapError(err))
	}
	out := make(map[string]domain.AssetRow, len(rows))
	for _, r := range rows {
		if r.TbLedger == nil {
			return nil, fmt.Errorf("postgres: asset %s has no ledger number", r.Asset)
		}
		out[r.Asset] = domain.AssetRow{Asset: r.Asset, TBLedger: uint32(*r.TbLedger), CreatedAt: platformpg.TimeOf(r.CreatedAt)}
	}
	return out, nil
}

// ListAssets returns the assets of a ledger in order of first use.
func (s *Store) ListAssets(ctx context.Context, ledgerName string) ([]domain.AssetRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListAssets(ctx, gen.ListAssetsParams{OrganizationID: org, Ledger: ledgerName})
	if err != nil {
		return nil, fmt.Errorf("postgres: list assets: %w", mapError(err))
	}
	out := make([]domain.AssetRow, 0, len(rows))
	for _, r := range rows {
		if r.TbLedger == nil {
			continue
		}
		out = append(out, domain.AssetRow{Asset: r.Asset, TBLedger: uint32(*r.TbLedger), CreatedAt: platformpg.TimeOf(r.CreatedAt), Accounts: r.Accounts})
	}
	return out, nil
}

// FindAccounts returns the existing accounts among the given pairs.
func (s *Store) FindAccounts(ctx context.Context, ledgerName string, keys []domain.AccountKey) ([]domain.AccountRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, nil
	}
	wanted := make(map[domain.AccountKey]bool, len(keys))
	addresses := make([]string, 0, len(keys))
	assets := make([]string, 0, len(keys))
	for _, k := range keys {
		wanted[k] = true
		addresses = append(addresses, string(k.Address))
		assets = append(assets, k.Asset)
	}
	rows, err := s.queries.FindAccounts(ctx, gen.FindAccountsParams{OrganizationID: org, Ledger: ledgerName, Addresses: addresses, Assets: assets})
	if err != nil {
		return nil, fmt.Errorf("postgres: find accounts: %w", mapError(err))
	}
	out := make([]domain.AccountRow, 0, len(keys))
	for i := range rows {
		r := &rows[i]
		row := accountRow(*r)
		if wanted[row.Key()] {
			out = append(out, row)
		}
	}
	return out, nil
}

// EnsureAccounts inserts the addresses and candidate accounts with ON CONFLICT DO NOTHING in one
// transaction, then returns the stored row for every candidate pair. A candidate that lost a race
// is replaced by the winner's row, ID, flags and code included (ADR-0006).
func (s *Store) EnsureAccounts(ctx context.Context, ledgerName string, candidates []domain.AccountRow) ([]domain.AccountRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	params := gen.InsertAccountsParams{OrganizationID: org, Ledger: ledgerName, CreatedAt: platformpg.Timestamptz(candidates[0].CreatedAt)}
	addresses := make([]string, 0, len(candidates))
	keys := make([]domain.AccountKey, 0, len(candidates))
	seen := map[ledger.Address]bool{}
	for i := range candidates {
		c := &candidates[i]
		params.Ids = append(params.Ids, platformpg.UUIDOf(c.ID))
		params.Addresses = append(params.Addresses, string(c.Address))
		params.Assets = append(params.Assets, c.Asset)
		params.TbLedgers = append(params.TbLedgers, int32(c.TBLedger))
		params.Codes = append(params.Codes, int32(c.Code))
		params.Limits = append(params.Limits, string(c.Limit))
		params.Histories = append(params.Histories, c.History)
		params.RulePatterns = append(params.RulePatterns, c.RulePattern)
		keys = append(keys, c.Key())
		if !seen[c.Address] {
			seen[c.Address] = true
			addresses = append(addresses, string(c.Address))
		}
	}
	err = s.inTx(ctx, func(q *gen.Queries) error {
		if err := q.InsertAddresses(ctx, gen.InsertAddressesParams{OrganizationID: org, Ledger: ledgerName, Addresses: addresses, CreatedAt: params.CreatedAt}); err != nil {
			return err
		}
		return q.InsertAccounts(ctx, params)
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: insert accounts: %w", mapError(err))
	}
	return s.FindAccounts(ctx, ledgerName, keys)
}

// AccountsByID returns accounts by ID.
func (s *Store) AccountsByID(ctx context.Context, ids []ledger.ID) ([]domain.AccountRow, error) {
	rows, err := s.queries.AccountsByID(ctx, platformpg.UUIDsOf(ids))
	if err != nil {
		return nil, fmt.Errorf("postgres: accounts by id: %w", mapError(err))
	}
	return accountRows(rows), nil
}

// withAccounts loads the accounts of the given addresses and attaches them.
func (s *Store) withAccounts(ctx context.Context, ledgerName string, rows []domain.AddressRow) ([]domain.AddressRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return rows, nil
	}
	names := make([]string, len(rows))
	index := make(map[ledger.Address]int, len(rows))
	for i, r := range rows {
		names[i] = string(r.Address)
		index[r.Address] = i
	}
	accounts, err := s.queries.AccountsOfAddresses(ctx, gen.AccountsOfAddressesParams{OrganizationID: org, Ledger: ledgerName, Addresses: names})
	if err != nil {
		return nil, fmt.Errorf("postgres: accounts of addresses: %w", mapError(err))
	}
	for k := range accounts {
		a := &accounts[k]
		row := accountRow(*a)
		i := index[row.Address]
		rows[i].Accounts = append(rows[i].Accounts, row)
	}
	return rows, nil
}

func addressRow(ledgerName, address string, metadata []byte, createdAt time.Time) (domain.AddressRow, error) {
	md, err := platformpg.DecodeMetadata(metadata)
	if err != nil {
		return domain.AddressRow{}, err
	}
	return domain.AddressRow{Ledger: ledgerName, Address: ledger.Address(address), Metadata: md, CreatedAt: createdAt}, nil
}

// ListAddresses returns a page of addresses with their accounts.
func (s *Store) ListAddresses(ctx context.Context, ledgerName string, filter domain.AddressFilter, after string, limit int) ([]domain.AddressRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListAddresses(ctx, gen.ListAddressesParams{
		OrganizationID: org,
		Ledger:         ledgerName,
		After:          after,
		Prefix:         platformpg.OptionalString(string(filter.Prefix)),
		Search:         platformpg.OptionalString(filter.Search),
		Asset:          platformpg.OptionalString(filter.Asset),
		PageSize:       int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list addresses: %w", mapError(err))
	}
	out := make([]domain.AddressRow, 0, len(rows))
	for _, r := range rows {
		row, err := addressRow(ledgerName, r.Address, r.Metadata, platformpg.TimeOf(r.CreatedAt))
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return s.withAccounts(ctx, ledgerName, out)
}

// SearchAddresses returns addresses starting with the text.
func (s *Store) SearchAddresses(ctx context.Context, ledgerName, text string, limit int) ([]domain.AddressRow, error) {
	return s.ListAddresses(ctx, ledgerName, domain.AddressFilter{Search: text}, "", limit)
}

// GetAddress returns an address with its accounts.
func (s *Store) GetAddress(ctx context.Context, ledgerName string, address ledger.Address) (domain.AddressRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return domain.AddressRow{}, err
	}
	r, err := s.queries.GetAddress(ctx, gen.GetAddressParams{OrganizationID: org, Ledger: ledgerName, Address: string(address)})
	if err != nil {
		return domain.AddressRow{}, fmt.Errorf("postgres: get address: %w", mapError(err))
	}
	row, err := addressRow(ledgerName, r.Address, r.Metadata, platformpg.TimeOf(r.CreatedAt))
	if err != nil {
		return domain.AddressRow{}, err
	}
	rows, err := s.withAccounts(ctx, ledgerName, []domain.AddressRow{row})
	if err != nil {
		return domain.AddressRow{}, err
	}
	return rows[0], nil
}

// ReplaceAddressMetadata replaces the metadata of an address.
func (s *Store) ReplaceAddressMetadata(ctx context.Context, ledgerName string, address ledger.Address, metadata map[string]string) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	raw, err := platformpg.EncodeMetadata(metadata)
	if err != nil {
		return err
	}
	n, err := s.queries.ReplaceAddressMetadata(ctx, gen.ReplaceAddressMetadataParams{OrganizationID: org, Ledger: ledgerName, Address: string(address), Metadata: raw})
	if err != nil {
		return fmt.Errorf("postgres: replace address metadata: %w", mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: replace address metadata: %w", ledger.ErrNotFound)
	}
	return nil
}

// AccountsUnder returns the accounts at or below a prefix.
func (s *Store) AccountsUnder(ctx context.Context, ledgerName string, prefix ledger.Address, limit int) ([]domain.AccountRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.AccountsUnder(ctx, gen.AccountsUnderParams{OrganizationID: org, Ledger: ledgerName, Prefix: string(prefix), MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("postgres: accounts under prefix: %w", mapError(err))
	}
	return accountRows(rows), nil
}

// Segments returns the child segments of a prefix.
func (s *Store) Segments(ctx context.Context, ledgerName string, prefix ledger.Address) ([]domain.SegmentRow, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.Segments(ctx, gen.SegmentsParams{OrganizationID: org, Ledger: ledgerName, Prefix: string(prefix)})
	if err != nil {
		return nil, fmt.Errorf("postgres: segments: %w", mapError(err))
	}
	out := make([]domain.SegmentRow, len(rows))
	for i, r := range rows {
		full := r.Segment
		if prefix != "" {
			full = string(prefix) + ":" + r.Segment
		}
		out[i] = domain.SegmentRow{Segment: r.Segment, Prefix: full, Accounts: r.Accounts, Assets: r.Assets, IsAccount: r.IsAccount, HasChildren: r.HasChildren}
	}
	return out, nil
}

// CountAddresses counts the addresses of a ledger and those created since a time.
func (s *Store) CountAddresses(ctx context.Context, ledgerName string, since time.Time) (total, recent int64, err error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return 0, 0, err
	}
	r, err := s.queries.CountAddresses(ctx, gen.CountAddressesParams{OrganizationID: org, Ledger: ledgerName, Since: platformpg.Timestamptz(since)})
	if err != nil {
		return 0, 0, fmt.Errorf("postgres: count addresses: %w", mapError(err))
	}
	return r.Total, r.Recent, nil
}
