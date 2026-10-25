package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// EnsureAssets allocates ledger numbers for new (ledger, asset) pairs with INSERT ... ON CONFLICT DO
// NOTHING and re-selects them, so concurrent first uses agree on one number (ADR-0005).
func (s *Store) EnsureAssets(ctx context.Context, ledgerName string, assets []ledger.Asset) (map[string]app.AssetRow, error) {
	names := make([]string, len(assets))
	for i, a := range assets {
		names[i] = a.String()
	}
	if err := s.queries.EnsureAssets(ctx, gen.EnsureAssetsParams{Ledger: ledgerName, Assets: names}); err != nil {
		return nil, fmt.Errorf("postgres: ensure assets: %w", mapError(err))
	}
	rows, err := s.queries.SelectAssets(ctx, gen.SelectAssetsParams{Ledger: ledgerName, Assets: names})
	if err != nil {
		return nil, fmt.Errorf("postgres: select assets: %w", mapError(err))
	}
	out := make(map[string]app.AssetRow, len(rows))
	for _, r := range rows {
		if r.TbLedger == nil {
			return nil, fmt.Errorf("postgres: asset %s has no ledger number", r.Asset)
		}
		out[r.Asset] = app.AssetRow{Asset: r.Asset, TBLedger: uint32(*r.TbLedger), CreatedAt: timeOf(r.CreatedAt)}
	}
	return out, nil
}

// ListAssets returns the assets of a ledger in order of first use.
func (s *Store) ListAssets(ctx context.Context, ledgerName string) ([]app.AssetRow, error) {
	rows, err := s.queries.ListAssets(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("postgres: list assets: %w", mapError(err))
	}
	out := make([]app.AssetRow, 0, len(rows))
	for _, r := range rows {
		if r.TbLedger == nil {
			continue
		}
		out = append(out, app.AssetRow{Asset: r.Asset, TBLedger: uint32(*r.TbLedger), CreatedAt: timeOf(r.CreatedAt), Accounts: r.Accounts})
	}
	return out, nil
}

// FindAccounts returns the existing accounts among the given pairs.
func (s *Store) FindAccounts(ctx context.Context, ledgerName string, keys []app.AccountKey) ([]app.AccountRow, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	wanted := make(map[app.AccountKey]bool, len(keys))
	addresses := make([]string, 0, len(keys))
	assets := make([]string, 0, len(keys))
	for _, k := range keys {
		wanted[k] = true
		addresses = append(addresses, string(k.Address))
		assets = append(assets, k.Asset)
	}
	rows, err := s.queries.FindAccounts(ctx, gen.FindAccountsParams{Ledger: ledgerName, Addresses: addresses, Assets: assets})
	if err != nil {
		return nil, fmt.Errorf("postgres: find accounts: %w", mapError(err))
	}
	out := make([]app.AccountRow, 0, len(keys))
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
func (s *Store) EnsureAccounts(ctx context.Context, ledgerName string, candidates []app.AccountRow) ([]app.AccountRow, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	params := gen.InsertAccountsParams{Ledger: ledgerName, CreatedAt: timestamptz(candidates[0].CreatedAt)}
	addresses := make([]string, 0, len(candidates))
	keys := make([]app.AccountKey, 0, len(candidates))
	seen := map[ledger.Address]bool{}
	for i := range candidates {
		c := &candidates[i]
		params.Ids = append(params.Ids, uuidOf(c.ID))
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
	err := s.inTx(ctx, func(q *gen.Queries) error {
		if err := q.InsertAddresses(ctx, gen.InsertAddressesParams{Ledger: ledgerName, Addresses: addresses, CreatedAt: params.CreatedAt}); err != nil {
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
func (s *Store) AccountsByID(ctx context.Context, ids []ledger.ID) ([]app.AccountRow, error) {
	rows, err := s.queries.AccountsByID(ctx, uuidsOf(ids))
	if err != nil {
		return nil, fmt.Errorf("postgres: accounts by id: %w", mapError(err))
	}
	return accountRows(rows), nil
}

// withAccounts loads the accounts of the given addresses and attaches them.
func (s *Store) withAccounts(ctx context.Context, ledgerName string, rows []app.AddressRow) ([]app.AddressRow, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	names := make([]string, len(rows))
	index := make(map[ledger.Address]int, len(rows))
	for i, r := range rows {
		names[i] = string(r.Address)
		index[r.Address] = i
	}
	accounts, err := s.queries.AccountsOfAddresses(ctx, gen.AccountsOfAddressesParams{Ledger: ledgerName, Addresses: names})
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

func addressRow(ledgerName, address string, metadata []byte, createdAt time.Time) (app.AddressRow, error) {
	md, err := decodeMetadata(metadata)
	if err != nil {
		return app.AddressRow{}, err
	}
	return app.AddressRow{Ledger: ledgerName, Address: ledger.Address(address), Metadata: md, CreatedAt: createdAt}, nil
}

// ListAddresses returns a page of addresses with their accounts.
func (s *Store) ListAddresses(ctx context.Context, ledgerName string, filter app.AddressFilter, after string, limit int) ([]app.AddressRow, error) {
	rows, err := s.queries.ListAddresses(ctx, gen.ListAddressesParams{
		Ledger:   ledgerName,
		After:    after,
		Prefix:   optionalString(string(filter.Prefix)),
		Search:   optionalString(filter.Search),
		Asset:    optionalString(filter.Asset),
		PageSize: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: list addresses: %w", mapError(err))
	}
	out := make([]app.AddressRow, 0, len(rows))
	for _, r := range rows {
		row, err := addressRow(ledgerName, r.Address, r.Metadata, timeOf(r.CreatedAt))
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return s.withAccounts(ctx, ledgerName, out)
}

// SearchAddresses returns addresses starting with the text.
func (s *Store) SearchAddresses(ctx context.Context, ledgerName, text string, limit int) ([]app.AddressRow, error) {
	return s.ListAddresses(ctx, ledgerName, app.AddressFilter{Search: text}, "", limit)
}

// GetAddress returns an address with its accounts.
func (s *Store) GetAddress(ctx context.Context, ledgerName string, address ledger.Address) (app.AddressRow, error) {
	r, err := s.queries.GetAddress(ctx, gen.GetAddressParams{Ledger: ledgerName, Address: string(address)})
	if err != nil {
		return app.AddressRow{}, fmt.Errorf("postgres: get address: %w", mapError(err))
	}
	row, err := addressRow(ledgerName, r.Address, r.Metadata, timeOf(r.CreatedAt))
	if err != nil {
		return app.AddressRow{}, err
	}
	rows, err := s.withAccounts(ctx, ledgerName, []app.AddressRow{row})
	if err != nil {
		return app.AddressRow{}, err
	}
	return rows[0], nil
}

// ReplaceAddressMetadata replaces the metadata of an address.
func (s *Store) ReplaceAddressMetadata(ctx context.Context, ledgerName string, address ledger.Address, metadata map[string]string) error {
	raw, err := encodeMetadata(metadata)
	if err != nil {
		return err
	}
	n, err := s.queries.ReplaceAddressMetadata(ctx, gen.ReplaceAddressMetadataParams{Ledger: ledgerName, Address: string(address), Metadata: raw})
	if err != nil {
		return fmt.Errorf("postgres: replace address metadata: %w", mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: replace address metadata: %w", ledger.ErrNotFound)
	}
	return nil
}

// AccountsUnder returns the accounts at or below a prefix.
func (s *Store) AccountsUnder(ctx context.Context, ledgerName string, prefix ledger.Address, limit int) ([]app.AccountRow, error) {
	rows, err := s.queries.AccountsUnder(ctx, gen.AccountsUnderParams{Ledger: ledgerName, Prefix: string(prefix), MaxRows: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("postgres: accounts under prefix: %w", mapError(err))
	}
	return accountRows(rows), nil
}

// Segments returns the child segments of a prefix.
func (s *Store) Segments(ctx context.Context, ledgerName string, prefix ledger.Address) ([]app.SegmentRow, error) {
	rows, err := s.queries.Segments(ctx, gen.SegmentsParams{Ledger: ledgerName, Prefix: string(prefix)})
	if err != nil {
		return nil, fmt.Errorf("postgres: segments: %w", mapError(err))
	}
	out := make([]app.SegmentRow, len(rows))
	for i, r := range rows {
		full := r.Segment
		if prefix != "" {
			full = string(prefix) + ":" + r.Segment
		}
		out[i] = app.SegmentRow{Segment: r.Segment, Prefix: full, Accounts: r.Accounts, Assets: r.Assets, IsAccount: r.IsAccount, HasChildren: r.HasChildren}
	}
	return out, nil
}

// CountAddresses counts the addresses of a ledger and those created since a time.
func (s *Store) CountAddresses(ctx context.Context, ledgerName string, since time.Time) (total, recent int64, err error) {
	r, err := s.queries.CountAddresses(ctx, gen.CountAddressesParams{Ledger: ledgerName, Since: timestamptz(since)})
	if err != nil {
		return 0, 0, fmt.Errorf("postgres: count addresses: %w", mapError(err))
	}
	return r.Total, r.Recent, nil
}
