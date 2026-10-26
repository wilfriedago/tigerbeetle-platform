// Package postgres implements the ledgers module's repositories on the pool of
// internal/platform/postgres. Its queries, in queries/, touch only the tables the module owns:
// ledgers, account_rules and event_types (plan section 4.7); sqlc generates gen/ from them.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// Store implements domain.LedgerRepository, domain.RuleRepository and domain.EventTypeRepository.
type Store struct {
	queries *gen.Queries
}

// NewStore builds the store on the process's pool.
func NewStore(db *platformpg.DB) *Store {
	return &Store{queries: gen.New(db.Pool())}
}

// CreateLedger inserts a ledger row.
func (s *Store) CreateLedger(ctx context.Context, l ledger.Ledger) (ledger.Ledger, error) {
	metadata, err := platformpg.EncodeMetadata(l.Metadata)
	if err != nil {
		return ledger.Ledger{}, err
	}
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return ledger.Ledger{}, err
	}
	row, err := s.queries.CreateLedger(ctx, gen.CreateLedgerParams{
		OrganizationID: org, Name: l.Name, Metadata: metadata, CreatedAt: platformpg.Timestamptz(l.CreatedAt),
	})
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("postgres: create ledger: %w", platformpg.MapError(err))
	}
	return ledgerOf(row)
}

// GetLedger loads a ledger row by name.
func (s *Store) GetLedger(ctx context.Context, name string) (ledger.Ledger, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return ledger.Ledger{}, err
	}
	row, err := s.queries.GetLedger(ctx, gen.GetLedgerParams{OrganizationID: org, Name: name})
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("postgres: get ledger: %w", platformpg.MapError(err))
	}
	return ledgerOf(row)
}

// ListLedgers returns ledgers in name order after the given name: all of them, or the named ones.
func (s *Store) ListLedgers(ctx context.Context, all bool, names []string, after string, limit int) ([]ledger.Ledger, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListLedgers(ctx, gen.ListLedgersParams{OrganizationID: org, After: after, AllLedgers: all, Names: names, PageSize: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("postgres: list ledgers: %w", platformpg.MapError(err))
	}
	out := make([]ledger.Ledger, 0, len(rows))
	for _, row := range rows {
		l, err := ledgerOf(row)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// ReplaceLedgerMetadata replaces the metadata of a ledger.
func (s *Store) ReplaceLedgerMetadata(ctx context.Context, name string, metadata map[string]string) error {
	raw, err := platformpg.EncodeMetadata(metadata)
	if err != nil {
		return err
	}
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	n, err := s.queries.ReplaceLedgerMetadata(ctx, gen.ReplaceLedgerMetadataParams{OrganizationID: org, Name: name, Metadata: raw})
	if err != nil {
		return fmt.Errorf("postgres: replace ledger metadata: %w", platformpg.MapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: replace ledger metadata: %w", ledger.ErrNotFound)
	}
	return nil
}

func ledgerOf(row gen.Ledger) (ledger.Ledger, error) {
	metadata, err := platformpg.DecodeMetadata(row.Metadata)
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("postgres: ledger %q: %w", row.Name, err)
	}
	return ledger.Ledger{Name: row.Name, Metadata: metadata, CreatedAt: platformpg.TimeOf(row.CreatedAt)}, nil
}

func ruleOf(pattern, limit string, history bool, code int32, createdAt, updatedAt pgtype.Timestamptz) ledger.Rule {
	return ledger.Rule{
		Pattern:   ledger.Pattern(pattern),
		Limit:     ledger.Limit(limit),
		History:   history,
		Code:      uint16(code),
		CreatedAt: platformpg.TimeOf(createdAt),
		UpdatedAt: platformpg.TimeOf(updatedAt),
	}
}

// ListRules returns the stored rules of a ledger.
func (s *Store) ListRules(ctx context.Context, ledgerName string) ([]ledger.Rule, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListRules(ctx, gen.ListRulesParams{OrganizationID: org, Ledger: ledgerName})
	if err != nil {
		return nil, fmt.Errorf("postgres: list rules: %w", platformpg.MapError(err))
	}
	out := make([]ledger.Rule, len(rows))
	for i, r := range rows {
		out[i] = ruleOf(r.Pattern, r.BalanceLimit, r.History, r.Code, r.CreatedAt, r.UpdatedAt)
	}
	return out, nil
}

// PutRule creates or replaces the rule for a pattern.
func (s *Store) PutRule(ctx context.Context, ledgerName string, rule ledger.Rule) (ledger.Rule, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return ledger.Rule{}, err
	}
	r, err := s.queries.PutRule(ctx, gen.PutRuleParams{
		OrganizationID: org,
		Ledger:         ledgerName,
		Pattern:        string(rule.Pattern),
		BalanceLimit:   string(rule.Limit),
		History:        rule.History,
		Code:           int32(rule.Code),
		CreatedAt:      platformpg.Timestamptz(rule.UpdatedAt),
	})
	if err != nil {
		return ledger.Rule{}, fmt.Errorf("postgres: put rule: %w", platformpg.MapError(err))
	}
	return ruleOf(r.Pattern, r.BalanceLimit, r.History, r.Code, r.CreatedAt, r.UpdatedAt), nil
}

// DeleteRule deletes a stored rule.
func (s *Store) DeleteRule(ctx context.Context, ledgerName string, pattern ledger.Pattern) error {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return err
	}
	n, err := s.queries.DeleteRule(ctx, gen.DeleteRuleParams{OrganizationID: org, Ledger: ledgerName, Pattern: string(pattern)})
	if err != nil {
		return fmt.Errorf("postgres: delete rule: %w", platformpg.MapError(err))
	}
	if n == 0 {
		return fmt.Errorf("postgres: delete rule %q: %w", pattern, ledger.ErrNotFound)
	}
	return nil
}

// ListEventTypes returns the stored event types in code order.
func (s *Store) ListEventTypes(ctx context.Context, ledgerName string) ([]ledger.EventType, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListEventTypes(ctx, gen.ListEventTypesParams{OrganizationID: org, Ledger: ledgerName})
	if err != nil {
		return nil, fmt.Errorf("postgres: list event types: %w", platformpg.MapError(err))
	}
	out := make([]ledger.EventType, len(rows))
	for i, r := range rows {
		out[i] = ledger.EventType{Name: r.Name, Code: uint16(r.Code), CreatedAt: platformpg.TimeOf(r.CreatedAt)}
	}
	return out, nil
}

// GetEventType returns a stored event type.
func (s *Store) GetEventType(ctx context.Context, ledgerName, name string) (ledger.EventType, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return ledger.EventType{}, err
	}
	r, err := s.queries.GetEventType(ctx, gen.GetEventTypeParams{OrganizationID: org, Ledger: ledgerName, Name: name})
	if err != nil {
		return ledger.EventType{}, fmt.Errorf("postgres: get event type: %w", platformpg.MapError(err))
	}
	return ledger.EventType{Name: r.Name, Code: uint16(r.Code), CreatedAt: platformpg.TimeOf(r.CreatedAt)}, nil
}

// CreateEventType inserts an event type. With code 0 it takes the next free code, retrying when a
// concurrent insert took the same one.
func (s *Store) CreateEventType(ctx context.Context, ledgerName, name string, code uint16) (ledger.EventType, error) {
	org, err := platformpg.TenantOf(ctx)
	if err != nil {
		return ledger.EventType{}, err
	}
	const attempts = 3
	for attempt := 0; ; attempt++ {
		c := int32(code)
		if code == 0 {
			next, err := s.queries.NextEventTypeCode(ctx, gen.NextEventTypeCodeParams{OrganizationID: org, Ledger: ledgerName})
			if err != nil {
				return ledger.EventType{}, fmt.Errorf("postgres: next event type code: %w", platformpg.MapError(err))
			}
			if next > 65535 {
				return ledger.EventType{}, fmt.Errorf("%w: no event type code left in this ledger", ledger.ErrInvalid)
			}
			c = next
		}
		r, err := s.queries.CreateEventType(ctx, gen.CreateEventTypeParams{OrganizationID: org, Ledger: ledgerName, Name: name, Code: c})
		var pgErr *pgconn.PgError
		if code == 0 && attempt < attempts && errors.As(err, &pgErr) && pgErr.ConstraintName == "event_types_code_unique" {
			continue
		}
		if err != nil {
			return ledger.EventType{}, fmt.Errorf("postgres: create event type: %w", platformpg.MapError(err))
		}
		return ledger.EventType{Name: r.Name, Code: uint16(r.Code), CreatedAt: platformpg.TimeOf(r.CreatedAt)}, nil
	}
}

// CountLedgers counts the ledgers of each organization, across organizations.
func (s *Store) CountLedgers(ctx context.Context, organizations []ledger.ID) (map[ledger.ID]int, error) {
	rows, err := s.queries.CountLedgers(ctx, platformpg.UUIDsOf(organizations))
	if err != nil {
		return nil, fmt.Errorf("postgres: count ledgers: %w", platformpg.MapError(err))
	}
	out := make(map[ledger.ID]int, len(rows))
	for _, r := range rows {
		out[platformpg.IDOf(r.OrganizationID)] = int(r.Ledgers)
	}
	return out, nil
}
