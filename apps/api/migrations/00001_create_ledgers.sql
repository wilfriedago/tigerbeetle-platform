-- Owned by the ledgers module (internal/modules/ledgers, plan section 4.7).
-- +goose Up
-- A ledger is a namespace for accounts and transactions (ADR-0004). Postgres holds its name and
-- metadata only; balances live in TigerBeetle (ADR-0002).
CREATE TABLE ledgers (
    name text PRIMARY KEY,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ledgers_name_format CHECK (name ~ '^[a-z0-9][a-z0-9_-]{0,62}$')
);

-- +goose Down
DROP TABLE ledgers;
