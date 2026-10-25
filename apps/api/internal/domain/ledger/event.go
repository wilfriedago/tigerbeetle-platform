package ledger

import (
	"fmt"
	"regexp"
	"time"
)

// EventType names the business event a transaction or hold records. Its code is written to the
// TigerBeetle transfer `code` (a non-zero u16) and never changes meaning, so event types are
// append-only (ADR-0007). The registry is per ledger, like every other name in a ledger.
type EventType struct {
	Name      string
	Code      uint16
	BuiltIn   bool
	CreatedAt time.Time
}

// DefaultEventType is used when a write names no event type. TigerBeetle rejects code 0.
var DefaultEventType = EventType{Name: "default", Code: 1, BuiltIn: true}

var eventTypeNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,62}$`)

// ValidateEventTypeName checks a name such as "card.authorized".
func ValidateEventTypeName(name string) error {
	if !eventTypeNamePattern.MatchString(name) {
		return fmt.Errorf("%w: event type %q must be lowercase, start with a letter, and use letters, digits, _ . or -, up to 63 characters", ErrInvalid, name)
	}
	return nil
}

// ValidateEventTypeCode checks a code chosen by a client. Code 1 belongs to the default event type.
func ValidateEventTypeCode(code int) error {
	if code < 2 || code > 65535 {
		return fmt.Errorf("%w: event type code must be between 2 and 65535; 1 is the default event type and TigerBeetle rejects 0", ErrInvalid)
	}
	return nil
}
