package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// LedgerSummary returns a ledger with its assets and address count.
func (s *Store) LedgerSummary(ctx context.Context, name string) (app.LedgerSummary, error) {
	row, err := s.queries.GetLedgerSummary(ctx, name)
	if err != nil {
		return app.LedgerSummary{}, fmt.Errorf("postgres: get ledger: %w", mapError(err))
	}
	l, err := toLedger(gen.Ledger{Name: row.Name, Metadata: row.Metadata, CreatedAt: row.CreatedAt})
	if err != nil {
		return app.LedgerSummary{}, err
	}
	return app.LedgerSummary{Ledger: l, Assets: row.Assets, Accounts: row.Accounts}, nil
}

// ListLedgers returns ledgers in name order after the given name.
func (s *Store) ListLedgers(ctx context.Context, after string, limit int) ([]app.LedgerSummary, error) {
	rows, err := s.queries.ListLedgers(ctx, gen.ListLedgersParams{After: after, PageSize: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("postgres: list ledgers: %w", mapError(err))
	}
	out := make([]app.LedgerSummary, 0, len(rows))
	for _, row := range rows {
		l, err := toLedger(gen.Ledger{Name: row.Name, Metadata: row.Metadata, CreatedAt: row.CreatedAt})
		if err != nil {
			return nil, err
		}
		out = append(out, app.LedgerSummary{Ledger: l, Assets: row.Assets, Accounts: row.Accounts})
	}
	return out, nil
}

// ReplaceLedgerMetadata replaces the metadata of a ledger.
func (s *Store) ReplaceLedgerMetadata(ctx context.Context, name string, metadata map[string]string) error {
	raw, err := encodeMetadata(metadata)
	if err != nil {
		return err
	}
	n, err := s.queries.ReplaceLedgerMetadata(ctx, gen.ReplaceLedgerMetadataParams{Name: name, Metadata: raw})
	if err != nil {
		return fmt.Errorf("postgres: replace ledger metadata: %w", mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: replace ledger metadata: %w", ledger.ErrNotFound)
	}
	return nil
}

func ruleOf(r gen.AccountRule) ledger.Rule {
	return ledger.Rule{
		Pattern:   ledger.Pattern(r.Pattern),
		Limit:     ledger.Limit(r.BalanceLimit),
		History:   r.History,
		Code:      uint16(r.Code),
		CreatedAt: timeOf(r.CreatedAt),
		UpdatedAt: timeOf(r.UpdatedAt),
	}
}

// ListRules returns the stored rules of a ledger.
func (s *Store) ListRules(ctx context.Context, ledgerName string) ([]ledger.Rule, error) {
	rows, err := s.queries.ListRules(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("postgres: list rules: %w", mapError(err))
	}
	out := make([]ledger.Rule, len(rows))
	for i, r := range rows {
		out[i] = ruleOf(gen.AccountRule{Pattern: r.Pattern, BalanceLimit: r.BalanceLimit, History: r.History, Code: r.Code, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
	}
	return out, nil
}

// PutRule creates or replaces the rule for a pattern.
func (s *Store) PutRule(ctx context.Context, ledgerName string, rule ledger.Rule) (ledger.Rule, error) {
	row, err := s.queries.PutRule(ctx, gen.PutRuleParams{
		Ledger:       ledgerName,
		Pattern:      string(rule.Pattern),
		BalanceLimit: string(rule.Limit),
		History:      rule.History,
		Code:         int32(rule.Code),
		CreatedAt:    timestamptz(rule.UpdatedAt),
	})
	if err != nil {
		return ledger.Rule{}, fmt.Errorf("postgres: put rule: %w", mapError(err))
	}
	return ruleOf(gen.AccountRule{Pattern: row.Pattern, BalanceLimit: row.BalanceLimit, History: row.History, Code: row.Code, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}), nil
}

// DeleteRule deletes a stored rule.
func (s *Store) DeleteRule(ctx context.Context, ledgerName string, pattern ledger.Pattern) error {
	n, err := s.queries.DeleteRule(ctx, gen.DeleteRuleParams{Ledger: ledgerName, Pattern: string(pattern)})
	if err != nil {
		return fmt.Errorf("postgres: delete rule: %w", mapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: delete rule %q: %w", pattern, ledger.ErrNotFound)
	}
	return nil
}

// ListEventTypes returns the stored event types in code order.
func (s *Store) ListEventTypes(ctx context.Context, ledgerName string) ([]app.EventTypeUsage, error) {
	rows, err := s.queries.ListEventTypes(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("postgres: list event types: %w", mapError(err))
	}
	out := make([]app.EventTypeUsage, len(rows))
	for i, r := range rows {
		out[i] = app.EventTypeUsage{EventType: ledger.EventType{Name: r.Name, Code: uint16(r.Code), CreatedAt: timeOf(r.CreatedAt)}}
	}
	return out, nil
}

// GetEventType returns a stored event type.
func (s *Store) GetEventType(ctx context.Context, ledgerName, name string) (ledger.EventType, error) {
	r, err := s.queries.GetEventType(ctx, gen.GetEventTypeParams{Ledger: ledgerName, Name: name})
	if err != nil {
		return ledger.EventType{}, fmt.Errorf("postgres: get event type: %w", mapError(err))
	}
	return ledger.EventType{Name: r.Name, Code: uint16(r.Code), CreatedAt: timeOf(r.CreatedAt)}, nil
}

// CreateEventType inserts an event type. With code 0 it takes the next free code, retrying when a
// concurrent insert took the same one.
func (s *Store) CreateEventType(ctx context.Context, ledgerName, name string, code uint16) (ledger.EventType, error) {
	const attempts = 3
	for attempt := 0; ; attempt++ {
		c := int32(code)
		if code == 0 {
			next, err := s.queries.NextEventTypeCode(ctx, ledgerName)
			if err != nil {
				return ledger.EventType{}, fmt.Errorf("postgres: next event type code: %w", mapError(err))
			}
			if next > 65535 {
				return ledger.EventType{}, fmt.Errorf("%w: no event type code left in this ledger", ledger.ErrInvalid)
			}
			c = next
		}
		r, err := s.queries.CreateEventType(ctx, gen.CreateEventTypeParams{Ledger: ledgerName, Name: name, Code: c})
		var pgErr *pgconn.PgError
		if code == 0 && attempt < attempts && errors.As(err, &pgErr) && pgErr.ConstraintName == "event_types_code_unique" {
			continue
		}
		if err != nil {
			return ledger.EventType{}, fmt.Errorf("postgres: create event type: %w", mapError(err))
		}
		return ledger.EventType{Name: r.Name, Code: uint16(r.Code), CreatedAt: timeOf(r.CreatedAt)}, nil
	}
}

// EventTypeUsage counts transactions and holds per event type.
func (s *Store) EventTypeUsage(ctx context.Context, ledgerName string) (map[string]int64, error) {
	rows, err := s.queries.EventTypeUsage(ctx, ledgerName)
	if err != nil {
		return nil, fmt.Errorf("postgres: event type usage: %w", mapError(err))
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.EventType] = r.Uses
	}
	return out, nil
}
