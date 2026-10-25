package ledger

import "time"

// Clock abstracts time so use cases are deterministic in tests.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function such as time.Now to Clock.
type ClockFunc func() time.Time

// Now calls the wrapped function.
func (f ClockFunc) Now() time.Time { return f() }
