package ledger

import (
	"encoding/base64"
	"fmt"
)

// Page sizes. Lists page with opaque cursors only: no offsets and no totals, because the tables
// grow append-only and offset scans get slower over time (ADR-0004).
const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

// PageSize clamps a requested page size.
func PageSize(requested int) int {
	switch {
	case requested <= 0:
		return DefaultPageSize
	case requested > MaxPageSize:
		return MaxPageSize
	default:
		return requested
	}
}

// EncodeCursor makes a sort key opaque. Clients must not build cursors; the encoding may change.
func EncodeCursor(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

// DecodeCursor reads a cursor EncodeCursor made. The empty cursor is the first page.
func DecodeCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", ErrBadCursor
	}
	return string(raw), nil
}

// ErrBadCursor answers a cursor this API did not hand out.
var ErrBadCursor = fmt.Errorf("%w: cursor is not one returned by this API", ErrInvalid)

// TrimPage returns the first limit items and the cursor of the next page. Repositories are asked
// for limit+1 rows so the last page needs no extra query.
func TrimPage[T any](rows []T, limit int, key func(T) string) (page []T, next string) {
	if len(rows) <= limit {
		return rows, ""
	}
	page = rows[:limit]
	return page, EncodeCursor(key(page[len(page)-1]))
}
