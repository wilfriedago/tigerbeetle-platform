package ledger

import "errors"

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
)
