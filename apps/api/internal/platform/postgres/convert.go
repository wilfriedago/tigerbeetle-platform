package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Conversions between kernel types and the pgx types sqlc generates. IDs are stored in uuid
// columns in big-endian byte order, the order of ledger.ID, so they copy straight across and
// ORDER BY id is time order (ADR-0002).

// UUIDOf converts an ID.
func UUIDOf(id ledger.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// UUIDsOf converts IDs.
func UUIDsOf(ids []ledger.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = UUIDOf(id)
	}
	return out
}

// NullableUUID maps the zero ID to NULL, for optional cursors and references.
func NullableUUID(id ledger.ID) pgtype.UUID {
	if id.IsZero() {
		return pgtype.UUID{}
	}
	return UUIDOf(id)
}

// IDOf converts a uuid column.
func IDOf(u pgtype.UUID) ledger.ID {
	return ledger.ID(u.Bytes)
}

// IDsOf converts a uuid array.
func IDsOf(us []pgtype.UUID) []ledger.ID {
	out := make([]ledger.ID, len(us))
	for i, u := range us {
		out[i] = IDOf(u)
	}
	return out
}

// OptionalID converts a nullable uuid column.
func OptionalID(u pgtype.UUID) *ledger.ID {
	if !u.Valid {
		return nil
	}
	id := IDOf(u)
	return &id
}

// Timestamptz converts a time.
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// OptionalTimestamptz maps nil to NULL.
func OptionalTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return Timestamptz(*t)
}

// TimeOf converts a timestamptz column to a time in UTC.
func TimeOf(t pgtype.Timestamptz) time.Time {
	return t.Time.UTC()
}

// OptionalTime converts a nullable timestamptz column.
func OptionalTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

// OptionalString maps "" to NULL.
func OptionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// StringOf maps NULL to "".
func StringOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// EncodeMetadata writes metadata as a jsonb value; nil is the empty object.
func EncodeMetadata(m map[string]string) ([]byte, error) {
	if m == nil {
		m = map[string]string{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("postgres: encode metadata: %w", err)
	}
	return b, nil
}

// DecodeMetadata reads a jsonb metadata value; an empty value is the empty map.
func DecodeMetadata(raw []byte) (map[string]string, error) {
	m := map[string]string{}
	if len(raw) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("postgres: decode metadata: %w", err)
	}
	return m, nil
}

// TenantOf returns the organization of the call, as the uuid every tenant query filters on
// (ADR-0026). A context without a tenant fails instead of reading across organizations.
func TenantOf(ctx context.Context) (pgtype.UUID, error) {
	t, err := ledger.RequireTenant(ctx)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("postgres: %w", err)
	}
	return UUIDOf(t.ID), nil
}
