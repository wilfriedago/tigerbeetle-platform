package ledger

import (
	"fmt"
	"time"
)

// MaxIdempotencyKeyLength bounds the Idempotency-Key header.
const MaxIdempotencyKeyLength = 255

// ValidateIdempotencyKey checks the Idempotency-Key of a write: 1 to 255 printable ASCII characters.
func ValidateIdempotencyKey(key string) error {
	if key == "" {
		return NewProblem(ErrInvalid, "missing-idempotency-key", "every write that moves money requires an Idempotency-Key header")
	}
	if len(key) > MaxIdempotencyKeyLength {
		return fmt.Errorf("%w: Idempotency-Key is longer than %d characters", ErrInvalid, MaxIdempotencyKeyLength)
	}
	for _, c := range key {
		if c < 0x21 || c > 0x7e {
			return fmt.Errorf("%w: Idempotency-Key must be printable ASCII without spaces", ErrInvalid)
		}
	}
	return nil
}

// Operation is the kind of write an idempotency record holds.
type Operation string

// Write operations. Each one is one TigerBeetle request with minted IDs (ADR-0007, ADR-0010).
const (
	OperationTransaction Operation = "transaction"
	OperationRevert      Operation = "revert"
	OperationHold        Operation = "hold"
	OperationCapture     Operation = "capture"
	OperationVoid        Operation = "void"
)

// RequestStatus is the state of an idempotency record. A record is pending, then committed,
// declined or failed, and never moves back (ADR-0007).
type RequestStatus string

// Record states. Declined means TigerBeetle refused a valid request (a balance limit, a resolved
// hold); failed covers everything else, such as a reconciler that could not recover the reason.
const (
	RequestPending   RequestStatus = "pending"
	RequestCommitted RequestStatus = "committed"
	RequestDeclined  RequestStatus = "declined"
	RequestFailed    RequestStatus = "failed"
)

// HoldStatus is the lifecycle state of a hold (ADR-0010).
type HoldStatus string

// Hold states. Expired is derived at read time: the hold is still open in Postgres but its timeout
// has passed on the cluster clock, so TigerBeetle has released it.
const (
	HoldOpen     HoldStatus = "open"
	HoldCaptured HoldStatus = "captured"
	HoldVoided   HoldStatus = "voided"
	HoldExpired  HoldStatus = "expired"
)

// MaxHoldTimeout is the largest hold timeout in seconds, the u32 range of the TigerBeetle field.
const MaxHoldTimeout = 1<<32 - 1

// HoldStatusAt derives the visible status of a hold stored as status at the given time.
func HoldStatusAt(stored HoldStatus, createdAt time.Time, timeoutSeconds uint32, now time.Time) HoldStatus {
	if stored != HoldOpen || timeoutSeconds == 0 {
		return stored
	}
	if now.After(createdAt.Add(time.Duration(timeoutSeconds) * time.Second)) {
		return HoldExpired
	}
	return HoldOpen
}
