-- Owned by the access module (ADR-0029).
-- +goose Up
-- An API key: tbp_<id>_<secret>. Only an HMAC-SHA-256 of the secret under AUTH_API_KEY_PEPPER is
-- kept. A revoked key stays, so its writes and grants keep naming it.
CREATE TABLE api_keys (
    id text PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id),
    name text NOT NULL,
    secret_hash bytea NOT NULL,
    expires_at timestamptz,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by_issuer text,
    created_by_subject text,
    created_by_name text,
    CONSTRAINT api_keys_id_format CHECK (id ~ '^[A-Za-z0-9]{8}$'),
    CONSTRAINT api_keys_name_format CHECK (name ~ '^[a-z0-9][a-z0-9-]{0,62}$')
);

-- A name names one active key of the organization.
CREATE UNIQUE INDEX api_keys_active_name ON api_keys (organization_id, name) WHERE revoked_at IS NULL;

-- A machine client: a confidential Keycloak client with a service account, whose tokens act for
-- the organization recorded here. The subject is the service account's user, the sub of its tokens.
CREATE TABLE machine_clients (
    client_id text PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id),
    keycloak_id uuid NOT NULL,
    subject text NOT NULL,
    name text NOT NULL,
    secret_set_at timestamptz NOT NULL DEFAULT now(),
    last_token_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by_issuer text,
    created_by_subject text,
    created_by_name text,
    CONSTRAINT machine_clients_name_format CHECK (name ~ '^[a-z0-9][a-z0-9-]{0,62}$')
);

CREATE UNIQUE INDEX machine_clients_active_name ON machine_clients (organization_id, name) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE machine_clients;
DROP TABLE api_keys;
