package domain

import (
	"context"
	"time"
)

// Session is a console session of the backend-for-frontend (ADR-0030).
type Session struct {
	// IDHash is the SHA-256 of the session ID the browser's cookie holds.
	IDHash  []byte
	Issuer  string
	Subject string
	// Tokens are the provider's tokens, sealed: only the access module can read them.
	Tokens          []byte
	AccessExpiresAt time.Time
	ExpiresAt       time.Time
}

// SessionRepository stores sessions. Sessions belong to a person, not to an organization.
type SessionRepository interface {
	CreateSession(ctx context.Context, s Session) error
	// SessionByHash returns ledger.ErrNotFound for an unknown or expired session.
	SessionByHash(ctx context.Context, idHash []byte, now time.Time) (Session, error)
	UpdateSessionTokens(ctx context.Context, idHash, tokens []byte, accessExpiresAt time.Time) error
	DeleteSession(ctx context.Context, idHash []byte) error
}
