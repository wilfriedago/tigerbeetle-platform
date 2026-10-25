package domain

import "time"

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
