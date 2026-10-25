package app

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Page sizes. Lists page with opaque cursors only: no offsets and no totals, because the tables
// grow append-only and offset scans get slower over time (ADR-0004).
const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

// pageSize clamps a requested page size.
func pageSize(requested int) int {
	switch {
	case requested <= 0:
		return DefaultPageSize
	case requested > MaxPageSize:
		return MaxPageSize
	default:
		return requested
	}
}

// encodeCursor makes a sort key opaque. Clients must not build cursors; the encoding may change.
func encodeCursor(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

func decodeCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", fmt.Errorf("%w: cursor is not one returned by this API", ledger.ErrInvalid)
	}
	return string(raw), nil
}

// decodeIDCursor reads a cursor holding a 32-hex ID.
func decodeIDCursor(cursor string) (ledger.ID, error) {
	key, err := decodeCursor(cursor)
	if err != nil || key == "" {
		return ledger.ID{}, err
	}
	id, err := ledger.ParseID(key)
	if err != nil {
		return ledger.ID{}, fmt.Errorf("%w: cursor is not one returned by this API", ledger.ErrInvalid)
	}
	return id, nil
}

func encodeWriteCursor(c WriteCursor) string {
	return encodeCursor(c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.Key)
}

func decodeWriteCursor(cursor string) (WriteCursor, error) {
	key, err := decodeCursor(cursor)
	if err != nil || key == "" {
		return WriteCursor{}, err
	}
	at, rest, ok := strings.Cut(key, "|")
	if !ok {
		return WriteCursor{}, fmt.Errorf("%w: cursor is not one returned by this API", ledger.ErrInvalid)
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return WriteCursor{}, fmt.Errorf("%w: cursor is not one returned by this API", ledger.ErrInvalid)
	}
	return WriteCursor{CreatedAt: t, Key: rest}, nil
}

// trimPage returns the first limit items and the cursor of the next page. Repositories are asked
// for limit+1 rows so the last page needs no extra query.
func trimPage[T any](rows []T, limit int, key func(T) string) (page []T, next string) {
	if len(rows) <= limit {
		return rows, ""
	}
	page = rows[:limit]
	return page, encodeCursor(key(page[len(page)-1]))
}
