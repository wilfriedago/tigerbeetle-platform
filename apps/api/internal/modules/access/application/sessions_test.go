package application

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// plainSealer seals by reversing the bytes, enough to tell sealed from not in these tests.
type plainSealer struct{}

func (plainSealer) Seal(p []byte) []byte { return reverse(p) }

func (plainSealer) Open(s []byte) ([]byte, error) {
	if len(s) == 0 || s[len(s)-1] != '{' {
		return nil, errors.New("not sealed")
	}
	return reverse(s), nil
}

func reverse(b []byte) []byte {
	out := make([]byte, len(b))
	for i := range b {
		out[len(b)-1-i] = b[i]
	}
	return out
}

type fakeIDP struct{ exchanged int }

func (*fakeIDP) AuthCodeURL(state, _, _ string) string { return "https://idp.test/auth?state=" + state }

func (f *fakeIDP) Exchange(context.Context, string, string, string) (Tokens, error) {
	f.exchanged++
	return Tokens{Access: "access", Refresh: "refresh", ID: "id", Expiry: time.Now().Add(5 * time.Minute)}, nil
}

func (*fakeIDP) Refresh(context.Context, string) (Tokens, error) { return Tokens{}, errors.New("no") }

func (*fakeIDP) EndSessionURL(string, string) string { return "" }

type fakeVerifier struct{}

func (fakeVerifier) Verify(context.Context, string) (ledger.Principal, error) {
	return ledger.Principal{Issuer: "idp", Subject: "ada"}, nil
}

type memSessions map[string]domain.Session

func (m memSessions) CreateSession(_ context.Context, s domain.Session) error {
	m[string(s.IDHash)] = s
	return nil
}

func (m memSessions) SessionByHash(_ context.Context, h []byte, now time.Time) (domain.Session, error) {
	s, ok := m[string(h)]
	if !ok || !now.Before(s.ExpiresAt) {
		return domain.Session{}, ledger.ErrNotFound
	}
	return s, nil
}

func (m memSessions) UpdateSessionTokens(context.Context, []byte, []byte, time.Time) error {
	return nil
}

func (m memSessions) DeleteSession(_ context.Context, h []byte) error {
	delete(m, string(h))
	return nil
}

func stateOf(t *testing.T, authURL string) string {
	t.Helper()
	const prefix = "https://idp.test/auth?state="
	if len(authURL) <= len(prefix) {
		t.Fatalf("auth URL %q", authURL)
	}
	return authURL[len(prefix):]
}

func TestSignIn(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	idp := &fakeIDP{}
	store := memSessions{}
	s := &sessions{idp: idp, tokens: fakeVerifier{}, sealer: plainSealer{}, store: store, clock: ledger.ClockFunc(func() time.Time { return now })}

	authURL, cookie := s.Begin("/acme/ledgers")
	state := stateOf(t, authURL)
	t.Run("a state from another sign-in is refused", func(t *testing.T) {
		if _, _, err := s.Complete(context.Background(), cookie, "code", state+"x"); !errors.Is(err, ledger.ErrUnauthenticated) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a tampered sign-in cookie is refused", func(t *testing.T) {
		raw, _ := base64.RawURLEncoding.DecodeString(cookie)
		raw[0] ^= 1
		if _, _, err := s.Complete(context.Background(), base64.RawURLEncoding.EncodeToString(raw[1:]), "code", state); !errors.Is(err, ledger.ErrUnauthenticated) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("an old sign-in is refused", func(t *testing.T) {
		late := &sessions{idp: idp, tokens: fakeVerifier{}, sealer: plainSealer{}, store: store, clock: ledger.ClockFunc(func() time.Time { return now.Add(time.Hour) })}
		if _, _, err := late.Complete(context.Background(), cookie, "code", state); !errors.Is(err, ledger.ErrUnauthenticated) {
			t.Fatalf("err = %v", err)
		}
	})
	if idp.exchanged != 0 {
		t.Fatalf("a refused sign-in exchanged its code %d times", idp.exchanged)
	}
	id, to, err := s.Complete(context.Background(), cookie, "code", state)
	if err != nil || to != "/acme/ledgers" || len(id) < 40 {
		t.Fatalf("Complete = %q %q %v", id, to, err)
	}
	if _, ok := store[id]; ok {
		t.Fatal("the session ID itself is stored; only its hash may be")
	}
	if token, err := s.AccessToken(context.Background(), id); err != nil || token != "access" {
		t.Fatalf("AccessToken = %q, %v", token, err)
	}
	if _, err := s.End(context.Background(), id, "/"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccessToken(context.Background(), id); !errors.Is(err, ledger.ErrUnauthenticated) {
		t.Fatalf("after End: %v", err)
	}
}
