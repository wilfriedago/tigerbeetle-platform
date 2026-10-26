package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// SessionLifetime is how long a console session lasts before the person signs in again, whatever
// the refreshes in between.
const SessionLifetime = 12 * time.Hour

// signInLifetime is how long a sign-in may take at the identity provider.
const signInLifetime = 10 * time.Minute

// refreshMargin renews the access token this long before it expires, so a call never carries a
// token that expires on the way.
const refreshMargin = 30 * time.Second

// errNoSession is the answer to a missing, expired or ended session: sign in again.
var errNoSession = ledger.NewProblem(ledger.ErrUnauthenticated, "unauthenticated", "the session has ended; sign in again")

// sessions runs the console's sign-in through the backend-for-frontend (ADR-0029). The browser
// only ever holds a random session ID; the tokens stay here, sealed.
type sessions struct {
	idp     IdentityProvider
	tokens  TokenVerifier
	sealer  Sealer
	store   domain.SessionRepository
	clock   ledger.Clock
	refresh singleflight.Group
}

// signIn is what the sign-in cookie carries between Begin and Complete, sealed.
type signIn struct {
	State    string    `json:"state"`
	Nonce    string    `json:"nonce"`
	Verifier string    `json:"verifier"`
	ReturnTo string    `json:"returnTo"`
	Expires  time.Time `json:"expires"`
}

// stored is the tokens a session keeps, sealed in its row.
type stored struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
	ID      string `json:"id"`
}

// Begin starts a sign-in: the URL to send the browser to, and the sealed value of the sign-in
// cookie that Complete checks.
func (s *sessions) Begin(returnTo string) (authURL, cookie string) {
	in := signIn{State: random(), Nonce: random(), Verifier: random() + random(), ReturnTo: returnTo, Expires: s.clock.Now().Add(signInLifetime)}
	raw, _ := json.Marshal(in)
	return s.idp.AuthCodeURL(in.State, in.Nonce, in.Verifier), base64.RawURLEncoding.EncodeToString(s.sealer.Seal(raw))
}

// Complete finishes a sign-in: it checks the state against the sign-in cookie, exchanges the code
// and opens a session. It returns the session ID for the session cookie and where to send the
// browser.
func (s *sessions) Complete(ctx context.Context, cookie, code, state string) (sessionID, returnTo string, err error) {
	failed := ledger.NewProblem(ledger.ErrUnauthenticated, "sign-in-failed", "the sign-in could not be completed; start again")
	raw, err := base64.RawURLEncoding.DecodeString(cookie)
	if err != nil {
		return "", "", failed
	}
	opened, err := s.sealer.Open(raw)
	if err != nil {
		return "", "", failed
	}
	var in signIn
	if json.Unmarshal(opened, &in) != nil || s.clock.Now().After(in.Expires) ||
		subtle.ConstantTimeCompare([]byte(in.State), []byte(state)) != 1 {
		return "", "", failed
	}
	tokens, err := s.idp.Exchange(ctx, code, in.Verifier, in.Nonce)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", failed, err)
	}
	p, err := s.tokens.Verify(ctx, tokens.Access)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", failed, err)
	}
	sealed, err := s.seal(tokens)
	if err != nil {
		return "", "", err
	}
	id := random()
	err = s.store.CreateSession(ctx, domain.Session{
		IDHash: hash(id), Issuer: p.Issuer, Subject: p.Subject, Tokens: sealed,
		AccessExpiresAt: tokens.Expiry, ExpiresAt: s.clock.Now().Add(SessionLifetime),
	})
	if err != nil {
		return "", "", fmt.Errorf("access: create session: %w", err)
	}
	return id, in.ReturnTo, nil
}

// AccessToken returns a valid access token of the session, refreshed when it is about to expire.
// Concurrent calls of one session share one refresh, because the provider rotates the refresh
// token and a second use of the old one would fail.
func (s *sessions) AccessToken(ctx context.Context, sessionID string) (string, error) {
	key := hash(sessionID)
	sess, err := s.store.SessionByHash(ctx, key, s.clock.Now())
	if errors.Is(err, ledger.ErrNotFound) {
		return "", errNoSession
	}
	if err != nil {
		return "", fmt.Errorf("access: session: %w", err)
	}
	tokens, err := s.open(sess.Tokens)
	if err != nil {
		return "", errNoSession
	}
	if s.clock.Now().Add(refreshMargin).Before(sess.AccessExpiresAt) {
		return tokens.Access, nil
	}
	v, err, _ := s.refresh.Do(string(key), func() (any, error) {
		fresh, err := s.idp.Refresh(ctx, tokens.Refresh)
		if err != nil {
			// Another replica may have refreshed first, and the provider rotated the token.
			if again, err2 := s.store.SessionByHash(ctx, key, s.clock.Now()); err2 == nil && again.AccessExpiresAt.After(s.clock.Now().Add(refreshMargin)) {
				if t, err3 := s.open(again.Tokens); err3 == nil {
					return t.Access, nil
				}
			}
			return "", errNoSession
		}
		if fresh.Refresh == "" {
			fresh.Refresh = tokens.Refresh
		}
		if fresh.ID == "" {
			fresh.ID = tokens.ID
		}
		sealed, err := s.seal(fresh)
		if err != nil {
			return "", err
		}
		if err := s.store.UpdateSessionTokens(ctx, key, sealed, fresh.Expiry); err != nil {
			return "", fmt.Errorf("access: update session: %w", err)
		}
		return fresh.Access, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// End ends the session here and returns where the browser ends it at the identity provider.
func (s *sessions) End(ctx context.Context, sessionID, postLogoutRedirect string) (logoutURL string, err error) {
	key := hash(sessionID)
	hint := ""
	if sess, err := s.store.SessionByHash(ctx, key, s.clock.Now()); err == nil {
		if t, err := s.open(sess.Tokens); err == nil {
			hint = t.ID
		}
	}
	if err := s.store.DeleteSession(ctx, key); err != nil {
		return "", fmt.Errorf("access: end session: %w", err)
	}
	return s.idp.EndSessionURL(hint, postLogoutRedirect), nil
}

func (s *sessions) seal(t Tokens) ([]byte, error) {
	raw, err := json.Marshal(stored{Access: t.Access, Refresh: t.Refresh, ID: t.ID})
	if err != nil {
		return nil, fmt.Errorf("access: seal tokens: %w", err)
	}
	return s.sealer.Seal(raw), nil
}

func (s *sessions) open(sealed []byte) (stored, error) {
	raw, err := s.sealer.Open(sealed)
	if err != nil {
		return stored{}, err
	}
	var t stored
	if err := json.Unmarshal(raw, &t); err != nil {
		return stored{}, err
	}
	return t, nil
}

// random returns 32 random bytes in URL-safe base64: session IDs, states, nonces and PKCE
// verifiers.
func random() string {
	b := make([]byte, 32)
	// crypto/rand.Read never fails on supported platforms (Go 1.24 and later).
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hash(sessionID string) []byte {
	sum := sha256.Sum256([]byte(sessionID))
	return sum[:]
}
