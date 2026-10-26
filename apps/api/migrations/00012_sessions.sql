-- Owned by the access module (ADR-0030).
-- +goose Up
-- A console session of the backend-for-frontend. The browser holds a random session ID in an
-- HttpOnly cookie; only its SHA-256 is stored, so a leaked table cannot be replayed as cookies.
-- The tokens are sealed with AUTH_SESSION_KEY (AES-256-GCM).
CREATE TABLE sessions (
    id_hash bytea PRIMARY KEY,
    issuer text NOT NULL,
    subject text NOT NULL,
    tokens bytea NOT NULL,
    access_expires_at timestamptz NOT NULL,
    -- The absolute end of the session; after it the person signs in again.
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_expires ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
