package application

import (
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
)

// decodeIDCursor reads a cursor holding a 32-hex ID.
func decodeIDCursor(cursor string) (ledger.ID, error) {
	key, err := ledger.DecodeCursor(cursor)
	if err != nil || key == "" {
		return ledger.ID{}, err
	}
	id, err := ledger.ParseID(key)
	if err != nil {
		return ledger.ID{}, ledger.ErrBadCursor
	}
	return id, nil
}

func encodeWriteCursor(c domain.WriteCursor) string {
	return ledger.EncodeCursor(c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.Key)
}

func decodeWriteCursor(cursor string) (domain.WriteCursor, error) {
	key, err := ledger.DecodeCursor(cursor)
	if err != nil || key == "" {
		return domain.WriteCursor{}, err
	}
	at, rest, ok := strings.Cut(key, "|")
	if !ok {
		return domain.WriteCursor{}, ledger.ErrBadCursor
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return domain.WriteCursor{}, ledger.ErrBadCursor
	}
	return domain.WriteCursor{CreatedAt: t, Key: rest}, nil
}

// parsePrefix accepts "users", "users:" or "" and returns the address form without the colon.
func parsePrefix(prefix string) (ledger.Address, error) {
	trimmed := strings.TrimSuffix(prefix, ":")
	if trimmed == "" {
		return "", nil
	}
	return ledger.ParseAddress(trimmed)
}
