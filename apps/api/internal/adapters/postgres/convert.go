package postgres

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/adapters/postgres/gen"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// IDs are stored in uuid columns in big-endian byte order, the order of ledger.ID, so they copy
// straight across and ORDER BY id is time order (ADR-0002).
func uuidOf(id ledger.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func uuidsOf(ids []ledger.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = uuidOf(id)
	}
	return out
}

// nullableUUID maps the zero ID to NULL, for optional cursors and references.
func nullableUUID(id ledger.ID) pgtype.UUID {
	if id.IsZero() {
		return pgtype.UUID{}
	}
	return uuidOf(id)
}

func idOf(u pgtype.UUID) ledger.ID {
	return ledger.ID(u.Bytes)
}

func idsOf(us []pgtype.UUID) []ledger.ID {
	out := make([]ledger.ID, len(us))
	for i, u := range us {
		out[i] = idOf(u)
	}
	return out
}

func optionalID(u pgtype.UUID) *ledger.ID {
	if !u.Valid {
		return nil
	}
	id := idOf(u)
	return &id
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func optionalTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return timestamptz(*t)
}

func timeOf(t pgtype.Timestamptz) time.Time {
	return t.Time.UTC()
}

func optionalTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// optionalString maps "" to NULL.
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func stringOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func encodeMetadata(m map[string]string) ([]byte, error) {
	if m == nil {
		m = map[string]string{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("postgres: encode metadata: %w", err)
	}
	return b, nil
}

func decodeMetadata(raw []byte) (map[string]string, error) {
	m := map[string]string{}
	if len(raw) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("postgres: decode metadata: %w", err)
	}
	return m, nil
}

func accountRow(a gen.Account) app.AccountRow {
	return app.AccountRow{
		ID:          idOf(a.ID),
		Ledger:      a.Ledger,
		Address:     ledger.Address(a.Address),
		Asset:       a.Asset,
		TBLedger:    uint32(a.TbLedger),
		Code:        uint16(a.Code),
		Limit:       ledger.Limit(a.BalanceLimit),
		History:     a.History,
		RulePattern: a.RulePattern,
		CreatedAt:   timeOf(a.CreatedAt),
	}
}

func accountRows(as []gen.Account) []app.AccountRow {
	out := make([]app.AccountRow, len(as))
	for i := range as {
		a := &as[i]
		out[i] = accountRow(*a)
	}
	return out
}
