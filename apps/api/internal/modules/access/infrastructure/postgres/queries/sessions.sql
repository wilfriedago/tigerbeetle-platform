-- sessions belong to a person, not to an organization: no tenant filter.

-- name: CreateSession :exec
INSERT INTO sessions (id_hash, issuer, subject, tokens, access_expires_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: SessionByHash :one
SELECT id_hash, issuer, subject, tokens, access_expires_at, expires_at
FROM sessions
WHERE id_hash = $1 AND expires_at > sqlc.arg(now);

-- name: UpdateSessionTokens :exec
UPDATE sessions
SET tokens = $2, access_expires_at = $3
WHERE id_hash = $1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id_hash = $1;
