package ledger

import (
	"errors"
	"time"
)

// Sentinel errors returned by use cases and adapters. Callers compare with errors.Is; the HTTP
// adapter maps them to RFC 9457 problem responses and never exposes the wrapped driver error.
var (
	// ErrNotFound means the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists means a create collided with an existing entity.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalid means the input violates a domain rule.
	ErrInvalid = errors.New("invalid")
	// ErrUnavailable means a dependency (Postgres or TigerBeetle) did not answer.
	ErrUnavailable = errors.New("dependency unavailable")
	// ErrRejected means a valid request that the ledger refuses: TigerBeetle declined the transfers
	// (a balance limit), or an Idempotency-Key was reused with a different body.
	ErrRejected = errors.New("rejected")
	// ErrConflict means the request conflicts with the current state of a resource: a write with
	// the same Idempotency-Key is still in flight, a hold is already resolved, a transaction is
	// already reverted.
	ErrConflict = errors.New("conflict")
)

// Problem is a domain error with a stable, kebab-case reason that clients can branch on, such as
// "exceeds-credits" or "request-in-flight". Kind is one of the sentinels above and decides the
// status class; errors.Is(problem, Kind) holds. The HTTP adapter turns the reason into the problem
// type "/problems/<reason>".
type Problem struct {
	Kind   error
	Reason string
	// Detail is a client-safe explanation of this occurrence.
	Detail string
	// Posting is the zero-based index of the posting that caused the problem, or -1. Zero is a
	// valid index, so constructors set it explicitly.
	Posting int
	// Results holds the TigerBeetle result of every posting of a declined chain, in order, such as
	// ["linked_event_failed", "exceeds_credits", "linked_event_failed"].
	Results []string
	// RetryAfter suggests when to retry, for a write still in flight.
	RetryAfter time.Duration
}

// NewProblem builds a problem that concerns no particular posting.
func NewProblem(kind error, reason, detail string) *Problem {
	return &Problem{Kind: kind, Reason: reason, Detail: detail, Posting: -1}
}

func (p *Problem) Error() string {
	if p.Detail == "" {
		return p.Kind.Error() + ": " + p.Reason
	}
	return p.Kind.Error() + ": " + p.Reason + ": " + p.Detail
}

// Unwrap lets errors.Is match the sentinel kind.
func (p *Problem) Unwrap() error {
	return p.Kind
}
