package application

import (
	"context"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// IdentityProvider is the console's OAuth client at the identity provider (ADR-0030); bootstrap
// adapts internal/platform/oidc to it.
type IdentityProvider interface {
	AuthCodeURL(ctx context.Context, state, nonce, verifier string) (string, error)
	Exchange(ctx context.Context, code, verifier, nonce string) (Tokens, error)
	Refresh(ctx context.Context, refreshToken string) (Tokens, error)
	EndSessionURL(idTokenHint, postLogoutRedirect string) string
}

// Tokens are what the identity provider issues to the console's client.
type Tokens struct {
	Access  string
	Refresh string
	ID      string
	Expiry  time.Time
}

// TokenVerifier checks an access token and returns its principal (internal/platform/oidc).
type TokenVerifier interface {
	Verify(ctx context.Context, raw string) (ledger.Principal, error)
}

// Sealer encrypts what the module stores or hands to a browser (internal/platform/sealed).
type Sealer interface {
	Seal(plaintext []byte) []byte
	Open(sealed []byte) ([]byte, error)
}

// LedgerCounter counts the ledgers of organizations, for the staff's organization pages; the
// ledgers module answers it, wired by bootstrap.
type LedgerCounter interface {
	CountLedgers(ctx context.Context, organizations []ledger.ID) (map[ledger.ID]int, error)
}
