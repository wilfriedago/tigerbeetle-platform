package ledger

import (
	"fmt"
	"regexp"
	"time"
)

// ledgerNamePattern mirrors the CHECK constraint on the ledgers table: lowercase, starts with a
// letter or digit, up to 63 characters.
var ledgerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// Ledger is a namespace for accounts and transactions (decision 2). Postgres stores the name and
// metadata; the money inside the ledger lives in TigerBeetle (decision 1).
type Ledger struct {
	Name      string
	Metadata  map[string]string
	CreatedAt time.Time
}

// ValidateLedgerName checks a ledger name against the naming rule.
func ValidateLedgerName(name string) error {
	if !ledgerNamePattern.MatchString(name) {
		return fmt.Errorf("%w: ledger name %q must match %s", ErrInvalid, name, ledgerNamePattern)
	}
	return nil
}
