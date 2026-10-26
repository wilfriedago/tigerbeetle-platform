package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/infrastructure/postgres/gen"
	platformpg "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/postgres"
)

// CreateSession stores a new session.
func (s *Store) CreateSession(ctx context.Context, sess domain.Session) error {
	err := s.queries.CreateSession(ctx, gen.CreateSessionParams{
		IDHash: sess.IDHash, Issuer: sess.Issuer, Subject: sess.Subject, Tokens: sess.Tokens,
		AccessExpiresAt: platformpg.Timestamptz(sess.AccessExpiresAt), ExpiresAt: platformpg.Timestamptz(sess.ExpiresAt),
	})
	if err != nil {
		return fmt.Errorf("postgres: create session: %w", platformpg.MapError(err))
	}
	return nil
}

// SessionByHash loads a session that has not expired.
func (s *Store) SessionByHash(ctx context.Context, idHash []byte, now time.Time) (domain.Session, error) {
	r, err := s.queries.SessionByHash(ctx, gen.SessionByHashParams{IDHash: idHash, Now: platformpg.Timestamptz(now)})
	if err != nil {
		return domain.Session{}, fmt.Errorf("postgres: session: %w", platformpg.MapError(err))
	}
	return domain.Session{
		IDHash: r.IDHash, Issuer: r.Issuer, Subject: r.Subject, Tokens: r.Tokens,
		AccessExpiresAt: platformpg.TimeOf(r.AccessExpiresAt), ExpiresAt: platformpg.TimeOf(r.ExpiresAt),
	}, nil
}

// UpdateSessionTokens stores refreshed tokens.
func (s *Store) UpdateSessionTokens(ctx context.Context, idHash, tokens []byte, accessExpiresAt time.Time) error {
	err := s.queries.UpdateSessionTokens(ctx, gen.UpdateSessionTokensParams{IDHash: idHash, Tokens: tokens, AccessExpiresAt: platformpg.Timestamptz(accessExpiresAt)})
	if err != nil {
		return fmt.Errorf("postgres: update session: %w", platformpg.MapError(err))
	}
	return nil
}

// DeleteSession ends a session.
func (s *Store) DeleteSession(ctx context.Context, idHash []byte) error {
	if err := s.queries.DeleteSession(ctx, idHash); err != nil {
		return fmt.Errorf("postgres: delete session: %w", platformpg.MapError(err))
	}
	return nil
}

// OrganizationsByAlias loads the organizations with the given aliases, in name order.
func (s *Store) OrganizationsByAlias(ctx context.Context, aliases []string) ([]domain.Organization, error) {
	rows, err := s.queries.OrganizationsByAlias(ctx, aliases)
	if err != nil {
		return nil, fmt.Errorf("postgres: organizations: %w", platformpg.MapError(err))
	}
	out := make([]domain.Organization, len(rows))
	for i := range rows {
		out[i] = organizationOf(gen.OrganizationByAliasRow(rows[i]))
	}
	return out, nil
}
